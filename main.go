package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Minimum difference between market and buylist price
const Tolerance = 0.75

// Minimum buylist price to consider
const Threshold = 0.1

// Maximum number of parallel requests
const MaxConcurrency = 8

const ConfigFile = "cfg.json"

var Config struct {
	ApiKey   string `json:"api_key"`
	UserName string `json:"user_name"`
}

type result struct {
	err error

	cardName     string
	cardSet      string
	price        float64
	buylistPrice float64
	isFoil       bool
	url          string
}

func processEntry(record []string) (ret result) {
	var err error
	cardName := strings.TrimSpace(record[1])
	cardSet := strings.TrimSpace(record[2])
	isFoil := strings.TrimSpace(record[5]) != ""
	buylistPrice, _ := strconv.ParseFloat(record[7], 64)
	if strings.HasPrefix(record[7], "$") {
		buylistPrice, _ = strconv.ParseFloat(record[7][1:], 64)
	}

	// skip small BL under this
	if buylistPrice < Threshold {
		return
	}

	// convert the CK name and set to CS versions
	cardName, cardSet, err = processRecord(cardName, cardSet)
	if err != nil {
		ret.err = fmt.Errorf("Error parsing %q - %q\n", record, err)
		return
	} else if cardName == "" || cardSet == "" {
		return
	}

	u, err := url.Parse(fmt.Sprintf("http://www.cardshark.com/API/%s/Get-Price.aspx", Config.UserName))
	if err != nil {
		ret.err = err
		return
	}
	q := u.Query()
	q.Set("apiKey", Config.ApiKey)
	q.Set("CardName", cardName)
	q.Set("CardSet", cardSet)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		ret.err = fmt.Errorf("Error retrieving %q - %q\n", record, err)
		return
	}
	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		ret.err = fmt.Errorf("Error reading %q - %q\n", record, err)
		return
	}
	if resp.StatusCode != 200 {
		ret.err = fmt.Errorf("Error requesting %q - %q\n", record, string(data))
		return
	}

	var response struct {
		Status    string `xml:"status"`
		Price     string `xml:"price"`
		FoilPrice string `xml:"foilprice"`
		Url       string `xml:"url"`
	}
	err = xml.Unmarshal(data, &response)
	if err != nil {
		ret.err = fmt.Errorf("Error decoding %q - %q\n", record, err)
		return
	}

	// check for missing prerelease cards and wrong foil prices
	isPrerelease := cardSet == "Prerelease Stamped"

	if response.Status != "valid card" {
		isConspiracy := cardSet == "Conspiracy Take the Crown"
		isSunCe := cardName == "Sun Ce, Young Conquerer"

		// skip errors for missing prerelease cards, CSP2-only
		// conspiracies, and a single p3k card
		if !isPrerelease && !isConspiracy && !isSunCe {
			ret.err = fmt.Errorf("Invalid record: (%s/%s) %q\n", cardName, cardSet, record)
		}
		return
	}

	// handle foil pricing differently for promos
	isPromo := strings.HasPrefix(cardSet, "Promotional")

	// this roundabout way is because some extremenly high prices have a ","
	// which prevents correct unmarshaling
	marketPrice, _ := strconv.ParseFloat(strings.Replace(response.Price, ",", "", -1), 64)
	foilPrice, _ := strconv.ParseFloat(strings.Replace(response.FoilPrice, ",", "", -1), 64)

	price := marketPrice
	if isFoil {
		price = foilPrice
	}
	// can't trust the promos, pick the lowest among the two prices
	if isPromo || isPrerelease {
		price = foilPrice
		if price-foilPrice > 0 {
			price = marketPrice
		}
	}

	ret.cardName = cardName
	ret.cardSet = cardSet
	ret.price = price
	ret.buylistPrice = buylistPrice
	ret.isFoil = isFoil
	ret.url = response.Url
	return
}

func run() int {
	l := log.New(os.Stderr, "", 0)

	if len(os.Args) < 2 {
		log.Fatal(fmt.Errorf("usage: <exe> <csv>"))
	}

	data, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(data, &Config)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	r := csv.NewReader(file)
	first, err := r.Read()
	if err == io.EOF {
		log.Fatal("Empty input file")
	}
	if err != nil {
		log.Fatal("Error reading record: " + err.Error())
	}
	if len(first) < 8 || (first[1] != "Card Name" &&
		first[2] != "CK_Modif_Set" && first[5] != "NF/F" &&
		first[7] != "BL_Value") {
		log.Fatal("Malformed input file")
	}

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	entries := 0
	records := make(chan []string)
	results := make(chan result)
	var wg sync.WaitGroup

	// Read from the records channel and block the subroutine until done
	// In this way you process entry up to MaxConcurrency at the same time
	for i := 0; i < MaxConcurrency; i++ {
		wg.Add(1)
		go func() {
			for record := range records {
				results <- processEntry(record)
			}
			wg.Done()
		}()
	}

	// Read from input file and queue records to be processed
	// Close channels and wait group when done
	// In case of error, wait for any remaining background routines
	go func() {
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				l.Printf("Error reading record: %s", err.Error())
				break
			}

			records <- record
		}
		close(records)

		wg.Wait()
		close(results)
	}()

	// Read from the result and apply any further logic
	for result := range results {
		if result.err != nil {
			l.Println(result.err)
			continue
		}

		if result.price > 0 && result.price <= Tolerance*result.buylistPrice {
			if entries == 0 {
				header := []string{
					"URL", "Name", "Set", "Foil", "Buylist Price", "CS Price", "Arb", "Spread",
				}
				w.Write(header)
			}
			foil := ""
			if result.isFoil {
				foil = "X"
			}
			buylistPriceStr := fmt.Sprintf("%0.2f", result.buylistPrice)
			priceStr := fmt.Sprintf("%0.2f", result.price)
			diff := fmt.Sprintf("%0.2f", result.buylistPrice-result.price)
			spread := fmt.Sprintf("%0.2f%%", 100*(result.buylistPrice-result.price)/result.price)

			record := []string{
				result.url,
				result.cardName,
				result.cardSet,
				foil,
				buylistPriceStr,
				priceStr,
				diff,
				spread,
			}
			err := w.Write(record)
			if err != nil {
				log.Fatalln("Error writing record to csv: ", err)
			}

			w.Flush()
			entries++
		}
	}

	return 0
}

func main() {
	os.Exit(run())
}

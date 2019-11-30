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
)

// Minimum difference between market and buylist price
const Tolerance = 0.75

// Minimum buylist price to consider
const Threshold = 0.1

const ConfigFile = "cfg.json"
const OutputFile = "output.csv"

var Config struct {
	ApiKey   string `json:"api_key"`
	UserName string `json:"user_name"`
}

func run() int {
	var prevSet string

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
	if len(first) < 7 || (first[1] != "Card Name" &&
		first[2] != "CK_Modif_Set" && first[5] != "NF/F" &&
		first[6] != "BL_Value") {
		log.Fatal("Malformed input file")
	}

	out, err := os.Create(OutputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	w := csv.NewWriter(out)
	defer w.Flush()

	header := []string{"URL", "Name", "Set", "Foil", "Buylist Price", "CS Price", "Arb", "Spread"}
	w.Write(header)

	u, err := url.Parse(fmt.Sprintf("http://www.cardshark.com/API/%s/Get-Price.aspx", Config.UserName))
	if err != nil {
		log.Fatal(err)
	}
	q := u.Query()
	q.Set("apiKey", Config.ApiKey)

	l := log.New(os.Stderr, "", 0)

	entry := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		entry++

		cardName := strings.TrimSpace(record[1])
		cardSet := strings.TrimSpace(record[2])
		isFoil := strings.TrimSpace(record[4]) != ""
		buylistPrice, _ := strconv.ParseFloat(record[6], 64)

		fmt.Printf("\033[2K\r[%05d] Processing '%s' card '%s' ", entry, cardSet, cardName)

		// skip small BL under this
		if buylistPrice < Threshold {
			continue
		}

		// convert the CK name and set to CS versions
		cardName, cardSet, err = processRecord(cardName, cardSet)
		if err != nil {
			l.Printf("Error parsing %q - %q\n", record, err)
			continue
		} else if cardName == "" || cardSet == "" {
			continue
		}

		q.Set("CardName", cardName)
		q.Set("CardSet", cardSet)
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			l.Printf("Error retrieving %q - %q\n", record, err)
			continue
		}
		data, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			l.Printf("Error reading %q - %q\n", record, err)
			continue
		}
		if resp.StatusCode != 200 {
			l.Printf("Error requesting %q - %q\n", record, string(data))
			continue
		}

		var response struct {
			Status    string `xml:"status"`
			Price     string `xml:"price"`
			FoilPrice string `xml:"foilprice"`
			Url       string `xml:"url"`
		}
		err = xml.Unmarshal(data, &response)
		if err != nil {
			l.Printf("Error decoding %q - %q\n", record, err)
			l.Printf("%s\n", u.String())
			continue
		}

		// check for missing prerelease cards and wrong foil prices
		isPrerelease := cardSet == "Prerelease Stamped"

		if response.Status != "valid card" {
			isConspiracy := cardSet == "Conspiracy Take the Crown"
			isSunCe := cardName == "Sun Ce, Young Conquerer"

			// ignore errors for missing prerelease cards, CSP2-only
			// conspiracies, and a single p3k card
			if !isPrerelease && !isConspiracy && !isSunCe {
				l.Printf("Invalid record: (%s/%s) %q\n", cardName, cardSet, record)
			}
			continue
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

		if price > 0 && price <= Tolerance*buylistPrice {
			foil := ""
			if isFoil {
				foil = "X"
			}
			buylistPriceStr := fmt.Sprintf("%0.2f", buylistPrice)
			priceStr := fmt.Sprintf("%0.2f", price)
			diff := fmt.Sprintf("%0.2f", buylistPrice-price)
			spread := fmt.Sprintf("%0.2f%%", 100*(buylistPrice-price)/price)

			record := []string{response.Url, cardName, cardSet, foil, buylistPriceStr, priceStr, diff, spread}
			err := w.Write(record)
			if err != nil {
				log.Fatalln("Error writing record to csv: ", err)
			}

			// Only flush whenever the set changes for sssssssspeed
			if prevSet != cardSet {
				w.Flush()
				prevSet = cardSet
			}
		}
	}
	fmt.Printf("\033\n")
	w.Flush()

	return 0
}

func main() {
	os.Exit(run())
}

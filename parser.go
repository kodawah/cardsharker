package main

import (
	"fmt"
	"strings"
)

// if cardName or cardSet are empty it's safe to skip
// otherwise error field will contain more info
func processRecord(cardName, cardSet string) (string, string, error) {
	// Skip basic lands, and strange cards
	if strings.HasPrefix(cardName, "Plains") ||
		strings.HasPrefix(cardName, "Island") ||
		strings.HasPrefix(cardName, "Swamp") ||
		strings.HasPrefix(cardName, "Mountain") ||
		strings.HasPrefix(cardName, "Forest") ||
		strings.HasPrefix(cardName, "Wastes") {
		return "", "", nil
	}
	if strings.Contains(cardName, "Token") ||
		strings.Contains(cardName, "Emblem") ||
		strings.Contains(cardName, "Oversized") ||
		strings.Contains(cardName, "Promo Plane") {
		return "", "", nil
	}

	// Drop qualifiers from the card name
	cardName = strings.Replace(cardName, " (Foil)", "", 1)
	cardName = strings.Replace(cardName, " (Foil - Planeswalker Deck)", "", 1)
	cardName = strings.Replace(cardName, " (Planeswalker Deck)", "", 1)
	cardName = strings.Replace(cardName, " (Planeswalker Deck Foil)", "", 1)
	cardName = strings.Replace(cardName, " (Spellslinger Starter Kit)", "", 1)
	cardName = strings.Replace(cardName, " (Welcome Deck)", "", 1)
	cardName = strings.Replace(cardName, " (Brawl Deck Card)", "", 1)
	if cardSet == "Throne of Eldraine Variants" {
		cardName = strings.Replace(cardName, " (Showcase)", "", 1)
		cardName = strings.Replace(cardName, " (Extended Art)", "", 1)
		cardName = strings.Replace(cardName, " (Borderless)", "", 1)
	}

	// Skip sets that make too much noise
	if strings.HasPrefix(cardSet, "Masterpiece Series") ||
		strings.HasPrefix(cardSet, "Un") {
		return "", "", nil
	}
	switch cardSet {
	case "Alpha", "Beta", "Collectors Ed", // these are present, but empty
		"Art Series",
		"Coldsnap Theme Decks",
		"Collectors Ed Intl",
		"Duels of the Planeswalkers",
		"Mystery Booster",
		"Promo Pack",
		"Ultimate Box Topper",
		"World Championships", // CS does not distinguish deck types anyway
		"War of the Spark JPN Planeswalkers":
		return "", "", nil
	}

	// Handle split cards, some editions treat the separator differently
	if strings.Contains(cardName, "//") {
		switch cardSet {
		case "Guilds of Ravnica", "Ravnica Allegiance", "Hour of Devastation",
			"Ultimate Masters", "Commander 2019", "Commander",
			"Duel Decks: Ajani Vs. Nicol Bolas", "Duel Decks: Izzet Vs. Golgari":
			s := strings.Split(cardName, " // ")
			cardName = s[0]
		case "Planar Chaos", "Timeshifted":
			cardName = strings.Replace(cardName, "// ", " ", 1)
		default:
			cardName = strings.Replace(cardName, "// ", "", 1)
		}
	}

	// Replace unsupported characters
	if strings.Contains(cardSet, "Duel Decks") ||
		strings.Contains(cardSet, "From the Vault") ||
		strings.Contains(cardSet, "Global Series") ||
		strings.Contains(cardSet, "Premium Deck Series") ||
		strings.Contains(cardSet, "Signature Spellbook") {
		cardSet = strings.Replace(cardSet, ":", "", 1)
		cardSet = strings.Replace(cardSet, "&", "and", 1)

		// the odd one out
		if strings.Contains(cardSet, "Annihilation") {
			cardSet += " (2014)"
		}
	}

	// custom sets, the DDA will need to it again because the deck variant is in the cardName
	if strings.HasPrefix(cardSet, "Duel Decks") {
		cardSet = strings.Replace(cardSet, "Elspeth Vs. Kiora", "Kiora Vs. Elspeth", 1)
		cardSet = strings.Replace(cardSet, "The Coalition", "the Coalition", 1)
		cardSet = strings.Replace(cardSet, "vs", "vs.", 1)
		cardSet = strings.Replace(cardSet, "Vs.", "vs.", 1)
	}

	// Convert edition names if needed
	entry, found := setMap[cardSet]
	if found {
		cardSet = entry
	}

	// Convert edition names for difficult cases
	switch cardSet {
	case "Guilds of Ravnica: Guild Kits", "Ravnica Allegiance: Guild Kits":
		// for some reason CS is extremely granular for this :@
		set := guildKitCards[cardName]
		cardSet = "Guild Kit " + set
	case "Duel Decks Anthology":
		// the deck variant is in the name for CK, but in the set for CS
		s := strings.Split(cardName, " (")
		if len(s) > 1 {
			cardName = s[0]
			interCardSet := strings.Replace(s[1][:len(s[1])-1], " - Foil", "", 1)
			interCardSet = strings.Replace(interCardSet, "Elspeth Vs. Kiora", "Kiora Vs. Elspeth", 1)
			interCardSet = strings.Replace(interCardSet, "The Coalition", "the Coalition", 1)
			interCardSet = strings.Replace(interCardSet, "vs", "vs.", 1)
			interCardSet = strings.Replace(interCardSet, "Vs.", "vs.", 1)
			cardSet = fmt.Sprintf("%s, %s", cardSet, interCardSet)
		}
	case "Promotional":
		// Good luck with this one
		s := strings.Split(cardName, " (")
		if len(s) > 1 {
			cardName = s[0]
			interCardSet := s[1][:len(s[1])-1]
			extra := ""
			if len(s) > 2 {
				interCardSet = s[1]        // fix previous trimming
				extra = s[2][:len(s[2])-2] // two '))' to drop
			}

			// Too noisy
			if strings.HasPrefix(interCardSet, "SDCC") ||
				strings.Contains(interCardSet, "MPS") ||
				strings.Contains(interCardSet, "JPN Alternate Art Prerelease Foil") {
				return "", "", nil
			}
			// Skip missing editions in CS
			for _, set := range skippablePromos {
				if interCardSet == set {
					return "", "", nil
				}
			}
			switch interCardSet {
			// CS supports only main JSS, excepct for 5 extra
			case "Junior Super Series Foil", "Junior Series Europe", "JSS Foil":
				cardSet = "Promotional Jr Super Series"
				tag := "Jr Super Series"
				switch cardName {
				case "Elvish Champion", "Glorious Anthem", "Soltari Priest", "Whirling Dervish":
					if interCardSet == "Junior Super Series Foil" {
						tag = "Scholarship Series"
					}
				case "Mad Auntie":
					cardSet = "" //skip
				}
				cardName = fmt.Sprintf("%s (%s)", cardName, tag)
			// JUDGE!!!
			case "Judge Foil":
				tag := "DCI Judge"
				cardSet = "Promotional DCI Judge"
				switch cardName {
				case "Bribery", "Command Tower", "Crucible of Worlds", "Dark Confidant",
					"Doubling Season", "Entomb", "Flusterstorm", "Goblin Welder",
					"Imperial Recruiter", "Karakas", "Karmic Guide", "Mana Crypt",
					"Noble Hierarch", "Sneak Attack", "Sword of Light and Shadow",
					"Swords to Plowshares", "Xiahou Dun, the One-Eyed":
					tag = "DCI Judge Foil"
				case "Vindicate":
					tag = "DCI Judge v1"
					if extra == "2013" {
						tag = "DCI Judge v2"
					}
				case "Wasteland":
					if extra == "2015" {
						cardSet = "" //skip
					}
				default:
					cardSet = "" //skip
				}
				cardName = fmt.Sprintf("%s (%s)", cardName, tag)
			// This is a mess, typo aside, cards fall in across multiple sets in CS and CK
			case "WPN Foil", "WPN 2011 Promo", "WPN 2011 Foil", "WPN - #51", "DCI Foil",
				"Gateway Foil", "M10 Game Day Foil", "Extended Art Foil", "Extended Art":
				tag := ""
				set := "Promotional Gateway"
				switch cardName {
				case "Wilt-Leaf Cavaliers":
					cardName = "" //skip
				case "Crystalline Sliver":
					tag = "FNM 2004"
					set = "Promotional Friday Night Magic"
				case "Underworld Dreams":
					tag = ""
					set = "Promotional Other"
				case "Black Sun's Zenith", "Blood Knight", "Bramblewood Paragon",
					"Doran, the Siege Tower", "Voidslime", "Urza's Factory", "Serra Avenger",
					"Liliana's Specter", "Imperious Perfect", "Groundbreaker",
					"Niv-Mizzet, the Firemind", "Mutavault", "Electrolyze":
					tag = promoTags[cardName]
					set = "Promotional Other"
				default:
					tag = gatewayTags[cardName]
					// Special case for 'Fling'
					if extra == "#69" {
						set = "" //skip
					}
				}
				if len(tag) > 0 {
					cardName = fmt.Sprintf("%s (%s)", cardName, tag)
				}
				cardSet = set
			// CS 'Prerelease Stamped' refers to pre M15 (pre)release promos
			case "Prerelease", "Prerelease Foil", "Prerelease foil",
				"Prerelease Foil - Non-English", "Prerelease Foil - non-English",
				"July 4 Prerelease", "Release Foil", "Release Promo Foil",
				"Launch Foil", "Launch Promo", "Launch Promo Foil",
				"Prerelease Foil - ELD", "Prerelease Foil - XLN":
				tag, found := prereleaseTags[cardName]
				if found {
					cardName = fmt.Sprintf("%s (%s)", cardName, tag)
				}
				cardSet = "Prerelease Stamped"

				// OF COURSE there is a single card breaking the pattern
				switch cardName {
				case "Ass Whuppin'":
					cardSet = "Promotional Other"
				case "Earl of Squirrel", "Magister of Worth":
					cardName = "" //skip
				}

			// Some specific cards are mapped to the set they belong
			case "Buy-A-Box", "Buy-A-Box Foil", "Buy-A-Box Non-Foil",
				"Buy-a-Box", "Buy-a-Box Foil":
				switch cardName {
				case "Nexus of Fate":
					cardSet = "Core Set 2019"
				case "Flusterstorm":
					cardSet = "Modern Horizons"
				case "The Haunt of Hightower":
					cardSet = "Ravnica Allegiance"
				case "Impervious Greatwurm":
					cardSet = "Guilds of Ravnica"
				case "Kenrith, the Returned King":
					cardSet = "Throne of Eldraine"
				default:
					cardSet = "Promotional Other"
					tag, found := promoTags[cardName]
					if !found {
						cardSet = "" //skip
					}
					if len(tag) > 0 {
						cardName = fmt.Sprintf("%s (%s)", cardName, tag)
					}
				}

			// And now the more sane ones
			case "2018 Gift Pack":
				cardSet = "Gift Pack"
			case "Textless", "Textless Foil", "Player Reward", "Player Reward Foil":
				tag := "(Player Rewards)"
				cardSet = "Promotional Player Rewards"
				switch cardName {
				case "Blightning", "Cancel", "Rampant Growth", "Terminate", "Lightning Bolt":
					tag = "(Player rewards)"
				case "Cryptic Command":
					cardName = "Cryptic command" //tyop
					tag = "(Player rewards)"
				case "Searing Blaze":
					tag = "(Player Rewards" //tyop
				case "Wrath of God":
					cardSet = "" //skip
				}
				cardName = fmt.Sprintf("%s %s", cardName, tag)
			case "Arena Foil", "Arena Promo":
				cardSet = "Promotional Arena League"
				if cardName == "Circle of Protection: Art" {
					return "", "", nil //skip
				}
				year, found := arenaYears[cardName]
				if !found {
					return "", "", fmt.Errorf("Arena not found: %s %s", cardName, cardSet)
				}
				cardName = fmt.Sprintf("%s (Arena %d)", cardName, year)
			case "FNM Foil":
				cardSet = "Promotional Friday Night Magic"
				tag, found := fnmYears[cardName]
				if !found {
					cardSet = "" //skip
				}
				if len(tag) > 0 {
					cardName = fmt.Sprintf("%s %s", cardName, tag)
				}
			default:
				cardSet = "Promotional Other"
				tag, found := promoTags[cardName]
				if !found {
					switch cardName {
					// Random cards
					case "Mutavault", "Progenitus", "Stoneforge Mystic":
						tag, cardName = "", "" //skip
					// Happy Holidays 2016+
					case "Bog Humbugs", "Thopter Pie Network", "Some Disassembly Required",
						"Mishra's Toy Workshop", "Goblin Sleigh Ride":
						cardName = "" //skip
					// Planeshift alt foils
					case "Skyship Weatherlight", "Ertai, the Corrupted", "Tahngarth, Talruum Hero":
						cardName += " (Alt. Art)"
						cardSet = "Planeshift"
					default:
						return "", "", fmt.Errorf("Promo not found: '%s' '%s'", cardName, cardSet)
					}
				}
				if len(tag) > 0 {
					cardName = fmt.Sprintf("%s (%s)", cardName, tag)
				}
				// whatever
				if cardName == "Flamerush Rider (alt art foil)" {
					cardSet = "Promotional other"
				}
			}
		}
	}

	// OK card set has been found, onto card name typos and peculiarities

	switch {
	// These cards only need replacement for some reprints (but not all)
	case cardName == "Altar of Dementia" && cardSet == "Tempest":
		cardName = "Altar Of Dementia"
	case cardName == "Furnace of Rath" && cardSet == "Tempest":
		cardName = "Furnace Of Rath"
	case cardName == "Commune with Nature" && cardSet == "Champions of Kamigawa":
		cardName = "Commune With Nature"
	case cardName == "Higure, the Still Wind" && cardSet == "Betrayers of Kamigawa":
		cardName = "Higure, The Still Wind"
	case cardName == "Flame-Kin Zealot" && cardSet == "Ravnica City of Guilds":
		cardName = "Flame kin Zealot"

	// CS hates magemarks
	case strings.Contains(cardName, "Magemark"):
		cardName = strings.Replace(cardName, "'", "’", 1)

	// CK tyops
	case strings.Contains(strings.ToLower(cardName), "okiba-gang"):
		cardName = "Okiba-Gang Shinobi"
		if cardSet == "Betrayers of Kamigawa" {
			cardName = "Okiba Gang Shinobi"
		}
	case cardName == "Will-O'-The-Wisp" || cardName == "Will-o'-the-Wisp":
		if cardSet == "Ninth Edition" {
			cardName = "Will o' the Wisp"
		} else if cardSet == "Masters 25" {
			cardName = "Will-o'-the-Wisp"
		} else {
			cardName = "Will O' The Wisp"
		}

	// Custom replacements
	case strings.Contains(cardName, "Lim-Dul"): // I wonder why CS hates limdul
		if cardName == "Lim-Dul the Necromancer" || cardName == "Lim-Dul's High Guard" {
			cardName = strings.Replace(cardName, "Lim-Dul", "Lim Dul", 1)
		} else if cardName == "Lim-Dul's Vault" && cardSet == "Commander 2013 Edition" {
			cardName = "Lim-Dûl's Vault"
		} else {
			cardName = strings.Replace(cardName, "Lim-Dul", "Lim Dûl", 1)
		}
	case strings.Contains(cardName, "Sakura-Tribe"): // I wonder why CS hates steve
		switch cardSet {
		case "Promotional Friday Night Magic":
			cardName = "Sakura - Tribe Elder (FNM 2009)"
		case "Archenemy", "Champions of Kamigawa", "World Championship Decks",
			"Promotional Jr Super Series", "Betrayers of Kamigawa":
			cardName = strings.Replace(cardName, "-", " ", -1)
		}

	// Guildgates
	case strings.Contains(cardName, "Guildgate") &&
		(cardSet == "Ravnica Allegiance" || cardSet == "Guilds of Ravnica"):
		s := strings.Split(cardName, " (")
		if strings.Contains(cardName, "Dimir") ||
			strings.Contains(cardName, "Izzet") ||
			strings.Contains(cardName, "Selesnya") {
			if strings.HasSuffix(cardName, "A)") {
				cardName = s[0] + " (a)"
			} else if strings.HasSuffix(cardName, "B)") {
				cardName = s[0] + " (b)"
			}
		} else {
			cardName = s[0]
		}
	// Urza's lands \o/
	case strings.HasPrefix(cardName, "Urza's") &&
		(cardSet == "Antiquities" || cardSet == "Chronicles"):
		entry, found = urzaLands[cardName][cardSet]
		if found {
			cardName = entry
		}

	// Not all Aethers are created equal..
	case strings.Contains(strings.ToLower(cardName), "aether") &&
		cardSet != "Commander 2018" &&
		cardSet != "Explorers of Ixalan" &&
		cardSet != "Iconic Masters":
		entry, found = aetherMess[cardName]
		if found {
			cardName = entry
		}

	// Last pass for the hard cases
	default:
		entry, found = anyVariant[cardName]
		if found {
			if len(entry) > 0 {
				cardName = entry
			}
		} else if strings.Contains(cardName, "-") {
			// SOME sets need dashes to be dropped, but NOT ALL
			for _, set := range dashingSets {
				if cardSet == set {
					cardName = strings.Replace(cardName, "-", " ", -1)
					break
				}
			}
		}
	}

	return cardName, cardSet, nil
}

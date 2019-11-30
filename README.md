# cardsharker

This script takes a csv file formatted as

```
CK_Key,Card Name,CK_Modif_Set,Set,Rarity,NF/F,MKT_Est,BL_Value
```

and uses the data to query CardShark database to obtain the best offers on cards.

The script requires to have a `cfg.json` file in the same folder containing your access information to the CardShark API.

```
{
    "api_key": "<key>",
    "user_name": "<name>"
}
```

It will output a second csv file containing

```
URL,Name,Set,Foil,Buylist Price,CS Price,Arb,Spread
```

Any error encountered will be output to stderr, while progress report will be printed on stdout.

Please don't run this too many times per day, as it puts servers under stress.

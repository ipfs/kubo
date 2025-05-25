package commands

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	logger "log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mrz1836/go-countries"

	"slices"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	config "github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	iface "github.com/ipfs/kubo/core/coreiface"
	peer "github.com/libp2p/go-libp2p/core/peer"
	cmds "github.com/stateless-minds/go-ipfs-cmds"
	orbitdb "github.com/stateless-minds/go-orbit-db"
	address "github.com/stateless-minds/go-orbit-db/address"
	orbitdb_iface "github.com/stateless-minds/go-orbit-db/iface"
	stores "github.com/stateless-minds/go-orbit-db/stores"
	"go.uber.org/zap"
)

const dbNameDemandSupply = "demand_supply"
const dbNameCitizenReputation = "citizen_reputation"
const dbNameIssue = "issue"
const dbNameEvent = "event"
const dbNameGift = "gift"
const dbNameRide = "ride"
const dbUser = "user"
const dbIncome = "income"
const dbWallet = "wallet"
const dbTransaction = "transaction"
const dbInflation = "inflation"
const dbPlan = "plan"
const dbSubscription = "subscription"
const transactionsPerPerson = 100

type User struct {
	ID            []byte                      `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`                       // Unique identifier for the user (should be a byte array)
	Name          string                      `mapstructure:"name" json:"name" validate:"uuid_rfc4122"`                     // Username or identifier for the user
	DisplayName   string                      `mapstructure:"display_name" json:"display_name" validate:"uuid_rfc4122"`     // Display name for the user
	CredentialIDs []webauthn.Credential       `mapstructure:"credential_ids" json:"credential_ids" validate:"uuid_rfc4122"` // List of credential IDs associated with the user
	Descriptor    map[string]map[int][]string `mapstructure:"descriptor" json:"descriptor" validate:"uuid_rfc4122"`         // Face descriptor for the user
	BusinessID    string                      `mapstructure:"business_id" json:"business_id" validate:"uuid_rfc4122"`       // Company business ID
	GovernmentID  string                      `mapstructure:"government_id" json:"government_id" validate:"uuid_rfc4122"`   // Government ID
	CountryCode   string                      `mapstructure:"country_code" json:"country_code" validate:"uuid_rfc4122"`     // Country Code
	Region        string                      `mapstructure:"region" json:"region" validate:"uuid_rfc4122"`                 // Region
}

type Wallet struct {
	ID           string `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`                     // Unique identifier for the user
	CountryCode  string `mapstructure:"country_code" json:"country_code" validate:"uuid_rfc4122"`   // Unique identifier for the country
	Balance      int    `mapstructure:"balance" json:"balance" validate:"uuid_rfc4122"`             // Balance of the user in cents
	Income       int    `mapstructure:"income" json:"income" validate:"uuid_rfc4122"`               // Recurring income of the user in cents
	LastReceived string `mapstructure:"last_received" json:"last_received" validate:"uuid_rfc4122"` // Date when basic income was last received
}

type Transaction struct {
	ID               string `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`                 // Unique identifier for the transaction
	SenderID         string `mapstructure:"sender_id" json:"sender_id" validate:"uuid_rfc4122"`     // Sender user id
	ReceiverID       string `mapstructure:"receiver_id" json:"receiver_id" validate:"uuid_rfc4122"` // Recipient user id
	ProductsServices []ProductService
	TotalCost        int       `mapstructure:"total_cost" json:"total_cost" validate:"uuid_rfc4122"` // Total cost of transaction
	Timestamp        time.Time `mapstructure:"timestamp" json:"timestamp" validate:"uuid_rfc4122"`   // Timestamp of the transaction
	Date             string    `mapstructure:"date" json:"date" validate:"uuid_rfc4122"`             // Date of the transaction in the format YY/MM
	Processed        bool      `mapstructure:"processed" json:"processed" validate:"uuid_rfc4122"`   // Flag if it was already processed by inflation indexer
}

type ProductService struct {
	ID     string `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"` // Unique identifier for the product
	Name   string `mapstructure:"name" json:"name" validate:"uuid_rfc4122"`
	Price  int    `mapstructure:"price" json:"price" validate:"uuid_rfc4122"`
	Amount int    `mapstructure:"amount" json:"amount" validate:"uuid_rfc4122"`
}

type Income struct {
	ID     string `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`       // Unique identifier for the income
	Amount int    `mapstructure:"amount" json:"amount" validate:"uuid_rfc4122"` // Amount of the income in cents
	Period string `mapstructure:"period" json:"period" validate:"uuid_rfc4122"` // Period the income is valid for
}

type Inflation struct {
	ID              string    `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`                           // Unique identifier for the income
	DiffPercentages []float64 `mapstructure:"diff_percentages" json:"diff_percentages" validate:"uuid_rfc4122"` // Amount of the income in cents
	Period          string    `mapstructure:"period" json:"period" validate:"uuid_rfc4122"`                     // Period the income is valid for
}

type Subscription struct {
	ID        string    `mapstructure:"_id" json:"_id" validate:"uuid_rfc4122"`               // Unique identifier for the transaction
	PlanID    string    `mapstructure:"plan_id" json:"plan_id" validate:"uuid_rfc4122"`       // Plan id
	UserID    string    `mapstructure:"user_id" json:"user_id" validate:"uuid_rfc4122"`       // User id
	Price     int       `mapstructure:"price" json:"price" validate:"uuid_rfc4122"`           // Price
	StartDate time.Time `mapstructure:"start_date" json:"start_date" validate:"uuid_rfc4122"` // Start date of subscription
	EndDate   time.Time `mapstructure:"end_date" json:"end_date" validate:"uuid_rfc4122"`     // End date of subscription
}

var OrbitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental orbit-db integration on ipfs.",
		ShortDescription: `
orbit db is a p2p database on top of ipfs node
`,
	},
	Subcommands: map[string]*cmds.Command{
		"kvput":                   OrbitPutKVCmd,
		"kvget":                   OrbitGetKVCmd,
		"kvdel":                   OrbitDelKVCmd,
		"docsput":                 OrbitPutDocsCmd,
		"docsget":                 OrbitGetDocsCmd,
		"docsquery":               OrbitQueryDocsCmd,
		"docsdel":                 OrbitDelDocsCmd,
		"create-country-accounts": OrbitCreateCountryAccountsCmd,
		"update-country-account":  OrbitUpdateCountryAccountCmd,
		"create-country-wallets":  OrbitCreateCountryWalletsCmd,
		"runindexer":              OrbitIndexerCmd,
		"delexpsubs":              OrbitExpSubsCmd,
	},
}

var OrbitPutKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Put value related to key",
		ShortDescription: `Key value store put
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		// read data passed as a file
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		_, err = store.Put(req.Context, key, data)
		if err != nil {
			return err
		}

		return nil
	},
}

var OrbitGetKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get value by key",
		ShortDescription: `Key value store get
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to get related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val := store.All()

			if err := res.Emit(&val); err != nil {
				return err
			}
		} else {
			val, err := store.Get(req.Context, key)
			if err != nil {
				return err
			}

			if err := res.Emit(&val); err != nil {
				return err
			}
		}

		return nil
	},
	Type: []byte{},
}

var OrbitDelKVCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Delete value by key",
		ShortDescription: `Key value store delete
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to delete related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectKV(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val := store.All()

			for i := range val {
				_, err := store.Delete(req.Context, i)
				if err != nil {
					return err
				}
			}
		} else {
			_, err := store.Delete(req.Context, key)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

// Generate random hyperplanes for LSH
func generateHyperplanes(dim, numHashes int) [][]float64 {
	rand.Seed(time.Now().UnixNano())
	hyperplanes := make([][]float64, numHashes)
	for i := 0; i < numHashes; i++ {
		hp := make([]float64, dim)
		for j := 0; j < dim; j++ {
			hp[j] = rand.NormFloat64() // Gaussian random values
		}
		hyperplanes[i] = hp
	}
	return hyperplanes
}

// Compute binary LSH hash signature from descriptor vector and hyperplanes
func computeLSHHash(descriptor []float64, hyperplanes [][]float64) []bool {
	signature := make([]bool, len(hyperplanes))
	for i, hp := range hyperplanes {
		dot := 0.0
		for j, val := range descriptor {
			dot += val * hp[j]
		}
		signature[i] = dot >= 0
	}
	return signature
}

// Convert bool slice to string of '0' and '1'
func boolSliceToString(bits []bool) string {
	s := make([]byte, len(bits))
	for i, bit := range bits {
		if bit {
			s[i] = '1'
		} else {
			s[i] = '0'
		}
	}
	return string(s)
}

// Optionally hash the binary string to get a fixed-length bucket key
func hashBand(band string) string {
	h := sha256.Sum256([]byte(band))
	return hex.EncodeToString(h[:])
}

func ConvertToFloatSlice(input []interface{}) ([]float64, error) {
	result := make([]float64, len(input))

	for i, val := range input {
		f, ok := val.(float64)
		if !ok {
			return nil, fmt.Errorf("element %d is type %T (not float64)", i, val)
		}
		result[i] = f
	}
	return result, nil
}

func uniqueStrings(input []string) []string {
	m := make(map[string]struct{})
	var result []string
	for _, s := range input {
		if _, exists := m[s]; !exists {
			m[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// SaveHyperplanes saves the hyperplanes matrix to a file
func SaveHyperplanes(filename string, hyperplanes [][]float64) error {
	// Check if file exists
	if _, err := os.Stat(filename); err == nil {
		// File exists, do not overwrite
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error (e.g., permission)
		return err
	}

	// File does not exist, create and write
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(hyperplanes); err != nil {
		return err
	}

	return nil
}

// LoadHyperplanes loads the hyperplanes matrix from a file
func LoadHyperplanes(filename string) ([][]float64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hyperplanes [][]float64
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&hyperplanes); err != nil {
		return nil, err
	}
	return hyperplanes, nil
}

var OrbitPutDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Put value related to key",
		ShortDescription: `Key value store put
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		// read data passed as a file
		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		dd := make(map[string]interface{})
		if err := json.Unmarshal(data, &dd); err != nil {
			return err
		}

		if _, ok := dd["descriptor"]; ok {
			d, ok := dd["descriptor"].(map[string]interface{})
			if !ok {
				return errors.New("descriptor not of correct type")
			}
			for key, descriptor := range d {
				desc, ok := descriptor.([]interface{})
				if ok {
					descFloat, err := ConvertToFloatSlice(desc)
					if err != nil {
						continue
					}

					descriptorsLSH := make(map[int][]string)

					var hyperplanes [][]float64

					hyperplanes, err = LoadHyperplanes("hyperplanes.gob")
					if err != nil {
						// Generate 32 random hyperplanes for LSH
						hyperplanes := generateHyperplanes(128, 32)

						// Save to file
						if err := SaveHyperplanes("hyperplanes.gob", hyperplanes); err != nil {
							log.Fatal("Failed to save hyperplanes:", err)
						}
					}

					// Compute binary LSH signature
					signature := computeLSHHash(descFloat, hyperplanes)

					// Split signature into 8 bands of 4 bits each and hash each band
					bands := 8
					rowsPerBand := len(signature) / bands
					for b := range bands {
						bandBits := signature[b*rowsPerBand : (b+1)*rowsPerBand]
						bandStr := boolSliceToString(bandBits)
						bucketKey := hashBand(bandStr)
						descriptorsLSH[b] = append(descriptorsLSH[b], bucketKey)
						descriptorsLSH[b] = uniqueStrings(descriptorsLSH[b])
					}
					d, ok := dd["descriptor"].(map[string]interface{})
					if !ok {
						return errors.New("descriptor not of correct type")
					}

					d[key] = descriptorsLSH
				}
			}
		}

		dbName := req.Arguments[0]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		_, err = store.Put(req.Context, dd)
		if err != nil {
			return err
		}

		return nil
	},
}

var OrbitGetDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get value by key",
		ShortDescription: `Key value store get
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to get related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			val, err := store.Get(req.Context, "", &orbitdb_iface.DocumentStoreGetOptions{CaseInsensitive: false})
			if err != nil {
				return err
			}

			if err := res.Emit(&val); err != nil {
				return err
			}
		} else {
			val, err := store.Get(req.Context, key, &orbitdb_iface.DocumentStoreGetOptions{CaseInsensitive: false})
			if err != nil {
				return err
			}

			if err := res.Emit(&val); err != nil {
				return err
			}
		}

		return nil
	},
	Type: []byte{},
}

func generateDummyTransactions(store orbitdb.DocumentStore) error {
	transactions := make([]Transaction, 400)
	for i := 0; i <= 199; i++ {
		// previous month
		transactions[i] = Transaction{
			ID:         uuid.NewString(),
			SenderID:   "7c7d5a26-3bf3-4547-9559-5c7725bc5562",
			ReceiverID: "c143d122-5880-4b11-b89c-b9afc8e49815",
			ProductsServices: []ProductService{
				{
					ID:     uuid.NewString(),
					Name:   "bread",
					Price:  100,
					Amount: 1,
				},
			},
			TotalCost: 100,
			Timestamp: time.Now(),
			Date:      strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa(int(time.Now().Month()-1)),
		}
		// current month
		transactions[199+i] = Transaction{
			ID:         uuid.NewString(),
			SenderID:   "7c7d5a26-3bf3-4547-9559-5c7725bc5562",
			ReceiverID: "c143d122-5880-4b11-b89c-b9afc8e49815",
			ProductsServices: []ProductService{
				{
					ID:     uuid.NewString(),
					Name:   "bread",
					Price:  200,
					Amount: 1,
				},
			},
			TotalCost: 200,
			Timestamp: time.Now(),
			Date:      strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa(int(time.Now().Month())),
		}
	}

	// Insert each transaction into the database
	processAndStoreTransactions(transactions, false, store)

	return nil
}

func processAndStoreTransactions(transactions []Transaction, processed bool, storeTr orbitdb.DocumentStore) error {
	timeBeforeTrProcessedLoop := time.Now()

	var wg sync.WaitGroup

	// Process and store transactions in parallel
	for _, transaction := range transactions {
		wg.Add(1)
		go func(transaction Transaction) {
			defer wg.Done()
			processAndStoreTransaction(transaction, processed, storeTr)
		}(transaction)
	}

	wg.Wait()

	logger.Printf("Processed transactions storing completed in: %f seconds", time.Since(timeBeforeTrProcessedLoop).Seconds())

	return nil
}

func processAndStoreTransaction(transaction Transaction, processed bool, storeTr orbitdb.DocumentStore) {
	transaction.Processed = processed

	// Directly marshal to JSON and store without unmarshaling
	processedJSON, err := json.Marshal(transaction)
	if err != nil {
		logger.Println("dbTransactions merge json marshal error: ", err)
		return
	}

	processedMap := make(map[string]interface{})

	err = json.Unmarshal(processedJSON, &processedMap)
	if err != nil {
		logger.Println("dbTransactions merge json unmarshal error: ", err)
		return
	}

	// Store the JSON directly
	_, err = storeTr.Put(context.Background(), processedMap)
	if err != nil {
		logger.Println("dbTransactions merge put error: ", err)
		return
	}
}

// aggregatePrices aggregates prices by product.
func aggregatePrices(transactions []Transaction) map[string][]int {
	pricesByProduct := make(map[string][]int)
	for i := range transactions {
		for _, n := range transactions[i].ProductsServices {
			pricesByProduct[n.Name] = append(pricesByProduct[n.Name], n.Price)
		}
	}
	return pricesByProduct
}

// calculateAveragePrice calculates the average price for a slice of prices.
func calculateAveragePrice(prices []int) float64 {
	sum := 0
	for _, p := range prices {
		sum += p
	}
	return float64(sum) / float64(len(prices))
}

// calculateInflation calculates inflation/deflation for matching products and returns a slice of inflation results.
func calculateInflation(prevMonth, currMonth []Transaction) []float64 {
	// Aggregate prices by product for both months.
	prevMonthPrices := aggregatePrices(prevMonth)
	currMonthPrices := aggregatePrices(currMonth)

	var inflationResults []float64

	// Iterate through products from previous month.
	for product, prevPrices := range prevMonthPrices {
		if currPrices, ok := currMonthPrices[product]; ok {
			// Calculate average prices.
			avgPrevPrice := calculateAveragePrice(prevPrices)
			avgCurrPrice := calculateAveragePrice(currPrices)

			// Calculate inflation/deflation.
			inflation := (avgCurrPrice - avgPrevPrice) / avgPrevPrice * 100
			inflationResults = append(inflationResults, inflation)
		} else {
			fmt.Printf("No current price found for %s\n", product)
		}
	}

	// Check for products only in current month.
	for product := range currMonthPrices {
		if _, ok := prevMonthPrices[product]; !ok {
			fmt.Printf("No previous price found for %s\n", product)
		}
	}

	return inflationResults
}

var OrbitExpSubsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Delete expired subscriptions",
		ShortDescription: `Expired subscriptions`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		timeStart := time.Now()

		// Define the log file name
		logFileName := "expired-subscriptions.log"

		// Open the log file in append mode
		logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Println("os.OpenFile error: ", err)
			return err
		}
		defer logFile.Close()

		// Create a new logger that writes to the log file
		logger := logger.New(logFile, "", logger.LstdFlags)

		logger.Println("Cleaner started")

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			logger.Println("cmdenv.GetApi error: ", err)
			return err
		}

		db, store, err := ConnectDocs(req.Context, dbSubscription, api, func(address string) {})
		if err != nil {
			logger.Println("dbTransaction ConnectDocs error: ", err)
			return err
		}

		defer db.Close()

		expiredSubs, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})
			// Define the format string
			layout := "2006-01-02T15:04:05.999999999-07:00" // Adjusted layout to include fractional seconds and offset
			endDate, err := time.Parse(layout, record["end_date"].(string))
			if err != nil {
				logger.Println("expired subs time parse error: ", err)
				return false, err
			}

			if time.Now().After(endDate) {
				return true, nil
			}

			return false, nil
		})

		if err != nil {
			logger.Println("expired subs query error: ", err)
			return err
		}

		// Convert the slice of interfaces to JSON string
		expiredSubsBytes, err := json.Marshal(&expiredSubs)
		if err != nil {
			logger.Println("expired subs json marshal error: ", err)
			return err
		}

		expiredSubsString := string(expiredSubsBytes)

		if expiredSubsString == "null" {
			logger.Println("no expired subs found")
		} else {
			var subscriptions []Subscription
			err = json.Unmarshal(expiredSubsBytes, &subscriptions)
			if err != nil {
				logger.Println("expired subs json unmarshal error: ", err)
				return err
			}

			for _, sub := range subscriptions {
				logger.Println("deleting expired subscription: ", sub.ID)
				_, err := store.Delete(req.Context, sub.ID)
				if err != nil {
					logger.Println("expired subs delete error: ", err)
					return err
				}
			}
		}

		logger.Printf("Delete expired subscriptions completed in: %f seconds", time.Since(timeStart).Seconds())

		return nil
	},
}

var OrbitCreateCountryAccountsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Create country accounts",
		ShortDescription: `Create country accounts`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			log.Fatal(err)
		}

		db, store, err := ConnectDocs(req.Context, dbUser, api, func(address string) {})
		if err != nil {
			log.Fatal(err)
		}

		defer db.Close()

		countriesMap := make(map[string]string)

		countries := countries.GetAll()
		for _, country := range countries {
			countriesMap[country.Alpha2] = country.Name
		}

		var users []map[string]interface{}

		for code, country := range countriesMap {
			user := User{
				Name:        country,
				DisplayName: country,
				ID:          protocol.URLEncodedBase64(uuid.NewString()),
				Descriptor:  nil,
				CountryCode: code,
			}

			userJSON, err := json.Marshal(user)
			if err != nil {
				log.Fatal(err)
			}

			userMap := make(map[string]interface{})

			err = json.Unmarshal(userJSON, &userMap)
			if err != nil {
				log.Fatal(err)
			}

			users = append(users, userMap)
		}

		values := make([]interface{}, len(users))
		for i, u := range users {
			values[i] = u
			_, err = store.Put(req.Context, values[i])
			if err != nil {
				log.Fatal(err)
			}
		}

		logger.Println("createCountryAccounts finished")

		return nil
	},
}

var OrbitUpdateCountryAccountCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Update country account",
		ShortDescription: `Update country account`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("country", true, false, "Country"),
		cmds.StringArg("government_id", true, false, "Government ID"),
		cmds.StringArg("name", true, false, "Name"),
		cmds.StringArg("descriptor", true, false, "Face descriptor"),
		cmds.StringArg("credential_id", true, false, "Credential ID"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			log.Fatal(err)
		}

		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		country := req.Arguments[0]
		governmentID := req.Arguments[1]
		name := req.Arguments[2]
		descriptor := req.Arguments[3]
		credentialID := req.Arguments[4]

		db, store, err := ConnectDocs(req.Context, dbUser, api, func(address string) {})
		if err != nil {
			log.Fatal(err)
		}

		defer db.Close()

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})
			if record["name"] == country {
				return true, nil
			}
			return false, nil
		})

		// Convert the slice of interfaces to JSON string
		dataBytes, err := json.Marshal(&q)
		if err != nil {
			log.Fatal(err)
		}

		dataString := string(dataBytes)

		if dataString == "null" {
			log.Fatal("no country account found")
		}

		users := []User{}
		err = json.Unmarshal(dataBytes, &users)
		if err != nil {
			log.Fatal(err)
		}

		descriptorsLSH := make(map[int][]string)

		var hyperplanes [][]float64

		hyperplanes, err = LoadHyperplanes("hyperplanes.gob")
		if err != nil {
			// Generate 32 random hyperplanes for LSH
			hyperplanes := generateHyperplanes(128, 32)

			// Save to file
			if err := SaveHyperplanes("hyperplanes.gob", hyperplanes); err != nil {
				log.Fatal(err)
			}
		}

		var descFloat []float64

		err = json.Unmarshal([]byte(descriptor), &descFloat)
		if err != nil {
			log.Fatal(err)
		}

		// Compute binary LSH signature
		signature := computeLSHHash(descFloat, hyperplanes)

		// Split signature into 8 bands of 4 bits each and hash each band
		bands := 8
		rowsPerBand := len(signature) / bands
		for b := range bands {
			bandBits := signature[b*rowsPerBand : (b+1)*rowsPerBand]
			bandStr := boolSliceToString(bandBits)
			bucketKey := hashBand(bandStr)
			descriptorsLSH[b] = append(descriptorsLSH[b], bucketKey)
			descriptorsLSH[b] = uniqueStrings(descriptorsLSH[b])
		}

		users[0].GovernmentID = governmentID

		users[0].Descriptor = make(map[string]map[int][]string)
		users[0].Descriptor[name] = descriptorsLSH
		users[0].CredentialIDs = []webauthn.Credential{
			{
				ID: []byte(credentialID),
			},
		}

		userJSON, err := json.Marshal(users[0])
		if err != nil {
			log.Fatal(err)
		}

		userMap := make(map[string]interface{})

		err = json.Unmarshal(userJSON, &userMap)
		if err != nil {
			log.Fatal(err)
		}

		_, err = store.Put(req.Context, userMap)
		if err != nil {
			log.Fatal(err)
		}

		logger.Println("updateCountryAccount finished")

		return nil
	},
}

var OrbitCreateCountryWalletsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Create country wallets",
		ShortDescription: `Create country wallets`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			log.Fatal(err)
		}

		dbu, storeUsers, err := ConnectDocs(req.Context, dbUser, api, func(address string) {})
		if err != nil {
			log.Fatal(err)
		}

		u, err := storeUsers.Query(req.Context, func(doc interface{}) (bool, error) {
			return true, nil
		})

		if err != nil {
			log.Fatal(err)
		}

		usersBytes, err := json.Marshal(u)
		if err != nil {
			log.Fatal(err)
		}

		usersString := string(usersBytes)

		if len(usersString) == 0 {
			log.Fatal("no country users found")
		}

		users := []User{}

		err = json.Unmarshal(usersBytes, &users)
		if err != nil {
			log.Fatal(err)
		}

		countriesMap := make(map[string]string)

		countries := countries.GetAll()
		for _, country := range countries {
			countriesMap[country.Alpha2] = country.Name
		}

		var wallets []map[string]interface{}

		for code := range countriesMap {
			countryWallet := &Wallet{
				CountryCode: code,
			}

			for _, user := range users {
				if user.CountryCode == countryWallet.CountryCode {
					countryWallet.ID = string(user.ID)
				}
			}

			countryWalletJSON, err := json.Marshal(countryWallet)
			if err != nil {
				log.Fatal(err)
			}

			countryWalletMap := make(map[string]interface{})

			err = json.Unmarshal(countryWalletJSON, &countryWalletMap)
			if err != nil {
				log.Fatal(err)
			}

			wallets = append(wallets, countryWalletMap)
		}

		dbu.Close()

		dbw, storeWallets, err := ConnectDocs(req.Context, dbWallet, api, func(address string) {})
		if err != nil {
			log.Fatal(err)
		}

		values := make([]interface{}, len(wallets))
		for i, u := range wallets {
			values[i] = u
			_, err = storeWallets.Put(req.Context, values[i])
			if err != nil {
				log.Fatal(err)
			}
		}

		dbw.Close()

		logger.Println("createCountryWallets finished")

		return nil
	},
}

var OrbitIndexerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Adjust basic income based on inflation/deflation",
		ShortDescription: `Inflation indexer`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		timeStart := time.Now()

		// Define the log file name
		logFileName := "indexer.log"

		// Open the log file in append mode
		logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Println("os.OpenFile error: ", err)
			return err
		}
		defer logFile.Close()

		// Create a new logger that writes to the log file
		logger := logger.New(logFile, "", logger.LstdFlags)

		logger.Println("Indexer started")

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			logger.Println("cmdenv.GetApi error: ", err)
			return err
		}

		dbTr, storeTr, err := ConnectDocs(req.Context, dbTransaction, api, func(address string) {})
		if err != nil {
			logger.Println("dbTransaction ConnectDocs error: ", err)
			return err
		}

		// logger.Println("generateDummyTransactions started")

		// timeBeforeGenerateDummyTransactions := time.Now()

		// err = generateDummyTransactions(storeTr)
		// if err != nil {
		// 	logger.Println("generateDummyTransactions error: ", err)
		// 	return err
		// }

		// dbTr.Close()

		// logger.Printf("generateDummyTransactions completed in: %f seconds", time.Since(timeBeforeGenerateDummyTransactions).Seconds())

		// return errors.New("force stop after generateDummyTransactions")

		timeBeforeQueryTransactions := time.Now()

		transactions, err := storeTr.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})

			// previous month
			if record["date"] == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month()-1)) {
				if record["processed"] == false {
					return true, nil
				}
				return false, nil
				// this month
			} else if record["date"] == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month())) {
				if record["processed"] == false {
					return true, nil
				}
				return false, nil
			}

			return false, nil
		})

		if err != nil {
			logger.Println("transactions query error: ", err)
			return err
		}

		dbTr.Close()

		logger.Printf("Query transactions completed in: %f seconds", time.Since(timeBeforeQueryTransactions).Seconds())
		// logger.Println("Transactions: ", transactions)

		// Convert the slice of interfaces to JSON string
		transactionsBytes, err := json.Marshal(&transactions)
		if err != nil {
			logger.Println("transactions marshal error: ", err)
			return err
		}

		transactionsString := string(transactionsBytes)
		transactionsPrevMonth := []Transaction{}
		transactionsCurrentMonth := []Transaction{}
		var transactionsTotal int
		var transactionsCollected int

		if transactionsString == "null" {
			logger.Println("no transactions found")
			logger.Println("check if income for next month exists")

			timeBeforeQueryIncome := time.Now()

			dbInc, storeInc, err := ConnectDocs(req.Context, dbIncome, api, func(address string) {})
			if err != nil {
				logger.Println("dbIncome connect error: ", err)
				return err
			}

			income, err := storeInc.Query(req.Context, func(e interface{}) (bool, error) {
				return true, nil
			})

			if err != nil {
				logger.Println("income query error: ", err)
				return err
			}

			logger.Printf("Query income completed in: %f seconds", time.Since(timeBeforeQueryIncome).Seconds())

			// Convert the slice of interfaces to JSON string
			incomeBytes, err := json.Marshal(&income)
			if err != nil {
				logger.Println("income json marshal error: ", err)
				return err
			}

			incomeString := string(incomeBytes)

			if incomeString == "null" {
				logger.Println("no income records found for current month")
				return nil
			} else {
				var incomes []Income
				err = json.Unmarshal(incomeBytes, &incomes)
				if err != nil {
					logger.Println("income json unmarshal error: ", err)
					return err
				}

				var currentIncome int
				var currentMonthExists bool
				var prevMonthExists bool
				var prevMonthAmount int

				for _, in := range incomes {
					if in.Period == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month()+1)) {
						logger.Println("income for next month exists, quitting")
						return errors.New("income for next month already exists")
					} else if in.Period == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month())) {
						currentMonthExists = true
						currentIncome = in.Amount
					} else if in.Period == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month()-1)) {
						prevMonthExists = true
						prevMonthAmount = in.Amount
					}
				}

				if !currentMonthExists {
					var amount int
					if prevMonthExists {
						amount = prevMonthAmount
					} else {
						amount = 200000
					}

					inc := Income{
						ID:     uuid.NewString(),
						Period: strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa(int(time.Now().Month())),
						Amount: amount,
					}

					incomeJSON, err := json.Marshal(inc)
					if err != nil {
						logger.Println("income json marshal error: ", err)
						return err
					}

					incomeMap := make(map[string]interface{})

					err = json.Unmarshal(incomeJSON, &incomeMap)
					if err != nil {
						logger.Println("income json unmarshal error: ", err)
						return err
					}

					_, err = storeInc.Put(req.Context, incomeMap)
					if err != nil {
						dbInc.Close()
						logger.Println("could not insert current month, err: ", err)
						return errors.New("could not insert current month")
					}

					logger.Println("current month income updated")
					return nil
				}

				dbInc.Close()

				if currentIncome > 0 {
					// query  inflation db with date this month
					logger.Println("query inflation db with date this month")
					timeBeforeQueryInflation := time.Now()

					dbInf, storeInf, err := ConnectDocs(req.Context, dbInflation, api, func(address string) {})
					if err != nil {
						logger.Println("dbInflation connect error: ", err)
						return err
					}

					inflation, err := storeInf.Query(req.Context, func(e interface{}) (bool, error) {
						record := e.(map[string]interface{})
						if record["period"] == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month())) {
							return true, nil
						}
						return false, nil
					})

					if err != nil {
						logger.Println("inflation query error: ", err)
						return err
					}

					dbInf.Close()

					logger.Printf("Query inflation completed in: %f seconds", time.Since(timeBeforeQueryInflation).Seconds())

					// Convert the slice of interfaces to JSON string
					inflationBytes, err := json.Marshal(&inflation)
					if err != nil {
						logger.Println("inflation json marshal error: ", err)
						return err
					}

					inflationString := string(inflationBytes)

					if inflationString == "null" {
						logger.Println("no inflation records found for current month")
						return nil
					} else {
						var inflations []Inflation
						err = json.Unmarshal(inflationBytes, &inflations)
						if err != nil {
							logger.Println("inflation json unmarshal error: ", err)
							return err
						}

						// if there is get diffPercentages and calculate new income
						timeBeforeDiffPercentagesLoop := time.Now()

						var diffs []float64
						for _, df := range inflations {
							diffs = append(diffs, df.DiffPercentages...)
						}

						var sum float64
						for _, value := range diffs {
							sum += value
						}

						logger.Printf("DiffPercentage loop logic completed in: %f seconds", time.Since(timeBeforeDiffPercentagesLoop).Seconds())

						percentageChange := sum / float64(len(inflations))
						logger.Printf("inflation/deflation: %v percent", percentageChange)

						// store new income in income db
						newIncomeAmount := float64(incomes[0].Amount) + (float64(incomes[0].Amount) * (percentageChange / 100))
						logger.Println("newIncomeAmount: ", newIncomeAmount)
						newIncome := Income{
							ID:     uuid.NewString(),
							Amount: int(newIncomeAmount),
							Period: strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa(int(time.Now().Month()+1)),
						}

						newIncomeJSON, err := json.Marshal(newIncome)
						if err != nil {
							logger.Println("new income json marshal error: ", err)
							return err
						}

						newIncomeMap := make(map[string]interface{})

						err = json.Unmarshal(newIncomeJSON, &newIncomeMap)
						if err != nil {
							logger.Println("new income json unmarshal error: ", err)
							return err
						}

						dbInc, storeInc, err := ConnectDocs(req.Context, dbIncome, api, func(address string) {})
						if err != nil {
							logger.Println("dbIncome connect error: ", err)
							return err
						}

						_, err = storeInc.Put(req.Context, newIncomeMap)
						if err != nil {
							logger.Println("new income db put error: ", err)
							return err
						}

						logger.Println("new income stored")

						dbInc.Close()
						return nil
					}
				}
				return nil
			}
		} else {
			var transactions []Transaction
			err = json.Unmarshal(transactionsBytes, &transactions)
			if err != nil {
				logger.Println("transactions json unmarshal error: ", err)
				return err
			}

			timeBeforeTransactionsLoop := time.Now()

			for _, tr := range transactions {
				transactionsTotal++
				if transactionsTotal <= transactionsPerPerson {
					transactionsCollected++
					if tr.Date == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month()-1)) {
						transactionsPrevMonth = append(transactionsPrevMonth, tr)
					} else {
						transactionsCurrentMonth = append(transactionsCurrentMonth, tr)
					}
				}
			}

			logger.Println("Transactions not processed yet: ", len(transactions))
			logger.Println("Transactions collected for processing: ", transactionsCollected)
			logger.Printf("Transactions loop logic completed in: %f seconds", time.Since(timeBeforeTransactionsLoop).Seconds())
		}

		if len(transactionsPrevMonth) > 0 && len(transactionsCurrentMonth) > 0 {
			timeBeforeMainLoop := time.Now()

			diffPercentages := calculateInflation(transactionsPrevMonth, transactionsCurrentMonth)

			logger.Printf("Main loop logic completed in: %f seconds", time.Since(timeBeforeMainLoop).Seconds())

			timeBeforeTrProcessedLoop := time.Now()

			processedTransactions := append(transactionsPrevMonth, transactionsCurrentMonth...)

			dbTr, storeTr, err := ConnectDocs(req.Context, dbTransaction, api, func(address string) {})
			if err != nil {
				logger.Println("dbTransactions merge connect error: ", err)
				return err
			}

			processAndStoreTransactions(processedTransactions, true, storeTr)

			dbTr.Close()

			logger.Printf("Processed transactions storing completed in: %f seconds", time.Since(timeBeforeTrProcessedLoop).Seconds())

			if len(diffPercentages) > 0 {
				// store diffPercentages in inflation db
				// with slice, date
				newInflation := Inflation{
					ID:              uuid.NewString(),
					DiffPercentages: diffPercentages,
					Period:          strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa(int(time.Now().Month())),
				}

				newInflationJSON, err := json.Marshal(newInflation)
				if err != nil {
					logger.Println("new inflation json marshal error: ", err)
					return err
				}

				var newInflationMap map[string]interface{}

				err = json.Unmarshal(newInflationJSON, &newInflationMap)
				if err != nil {
					logger.Println("new inflation json unmarshal error: ", err)
					return err
				}

				dbInf, storeInf, err := ConnectDocs(req.Context, dbInflation, api, func(address string) {})
				if err != nil {
					logger.Println("dbInflation connect error: ", err)
					return err
				}

				_, err = storeInf.Put(req.Context, newInflationMap)
				if err != nil {
					logger.Println("new inflation db put error: ", err)
					return err
				}

				logger.Println("new inflation stored")

				dbInf.Close()
			}
		} else if len(transactionsPrevMonth) == 0 {
			logger.Println("no unprocessed transactions for previous month found")
		} else if len(transactionsCurrentMonth) == 0 {
			logger.Println("no unprocessed transactions for current month found")
		}

		logger.Printf("Indexer completed in: %f seconds", time.Since(timeStart).Seconds())

		return nil
	},
}

func isDescriptorMatch(stored map[int][]string, newBuckets map[int]string, threshold int) bool {
	matches := 0
	for band, newKey := range newBuckets {
		// Check if this band exists in stored data
		storedKeys, ok := stored[band]
		if !ok {
			continue // No data for this band
		}

		// Check if newKey exists in storedKeys
		if slices.Contains(storedKeys, newKey) {
			matches++ // Match found for this band
		}

		logger.Println("matches: ", matches)

		// Early exit if threshold met
		if matches >= threshold {
			return true
		}
	}

	return matches >= threshold
}

var OrbitQueryDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Query docs by key and value",
		ShortDescription: `Query docs store by key and value
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to query"),
		cmds.StringArg("value", true, false, "Value to query"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]
		value := req.Arguments[2]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		// w, err := store.Query(req.Context, func(e interface{}) (bool, error) {
		// 	record := e.(map[string]interface{})
		// 	if record["country_code"] == "BG" {
		// 		return true, nil
		// 	}

		// 	return false, nil
		// })

		// if err != nil {
		// 	return err
		// }

		// // Convert the slice of interfaces to JSON string
		// walletsBytes, err := json.Marshal(&w)
		// if err != nil {
		// 	return err
		// }

		// var wallets []Wallet

		// err = json.Unmarshal(walletsBytes, &wallets)
		// if err != nil {
		// 	return err
		// }

		// for _, wallet := range wallets {
		// 	wallet.Balance = 1000000
		// 	walletJSON, err := json.Marshal(wallet)
		// 	if err != nil {
		// 		return err
		// 	}

		// 	walletMap := make(map[string]interface{})

		// 	err = json.Unmarshal(walletJSON, &walletMap)
		// 	if err != nil {
		// 		return err
		// 	}

		// 	_, err = store.Put(req.Context, walletMap)
		// 	if err != nil {
		// 		return err
		// 	}
		// }

		// db.Close()

		// return errors.New("top up BG wallets")

		var index int

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})
			if (key == "all" && value == "") || key == "descriptor" {
				return true, nil
			} else if key == "all" && strings.Contains(value, "range") {
				// value is range
				indexRange := strings.Split(value, "=")
				indexes := strings.Split(indexRange[1], "-")
				start, err := strconv.Atoi(indexes[0])
				if err != nil {
					return false, err
				}
				end, err := strconv.Atoi(indexes[1])
				if err != nil {
					return false, err
				}

				if index >= start && index <= end {
					index++
					return true, nil
				} else {
					index++
					return false, nil
				}
			} else if strings.Contains(value, ",") {
				values := strings.Split(value, ",")
				var start int
				var end int
				for i, val := range values {
					if strings.Contains(val, "range") {
						// value is range
						indexRange := strings.Split(value, "=")
						indexes := strings.Split(indexRange[1], "-")
						start, err = strconv.Atoi(indexes[0])
						if err != nil {
							return false, err
						}
						end, err = strconv.Atoi(indexes[1])
						if err != nil {
							return false, err
						}

						values = slices.Delete(values, i, i+1)
					}
				}

				if strings.Contains(key, ",") {
					keys := strings.Split(key, ",")
					for _, k := range keys {
						record, ok := record[k].(string)
						if !ok {
							return false, nil
						}

						for _, v := range values {
							if record == v {
								if index >= start && index <= end {
									index++
									return true, nil
								} else {
									index++
									return false, nil
								}
							}
						}
					}
				} else {
					for _, v := range values {
						if record[key] == v {
							if index >= start && index <= end {
								index++
								return true, nil
							} else {
								index++
								return false, nil
							}
						}
					}
				}
			} else if strings.Contains(key, ",") {
				keys := strings.Split(key, ",")
				for _, k := range keys {
					record, ok := record[k].(string)
					if !ok {
						return false, nil
					}

					if record == value {
						return true, nil
					}
				}
			} else if record[key] == value {
				return true, nil
			}
			return false, nil
		})

		if err != nil {
			return err
		}

		// Convert the slice of interfaces to JSON string
		dataBytes, err := json.Marshal(&q)
		if err != nil {
			return err
		}

		dataString := string(dataBytes)

		if dataString == "null" {
			if err := res.Emit(nil); err != nil {
				return err
			}
		} else {
			if key == "descriptor" {
				users := []User{}
				err := json.Unmarshal(dataBytes, &users)
				if err != nil {
					return err
				}

				descriptor := []float64{}

				err = json.Unmarshal([]byte(value), &descriptor)
				if err != nil {
					return err
				}

				var hyperplanes [][]float64

				newBuckets := make(map[int]string)
				hyperplanes, err = LoadHyperplanes("hyperplanes.gob")
				if err != nil {
					// Generate 32 random hyperplanes for LSH
					hyperplanes := generateHyperplanes(128, 32)

					// Save to file
					if err := SaveHyperplanes("hyperplanes.gob", hyperplanes); err != nil {
						log.Fatal("Failed to save hyperplanes:", err)
					}
				}

				for _, user := range users {
					// Compute binary LSH signature
					signature := computeLSHHash(descriptor, hyperplanes)
					// Split signature into 8 bands of 4 bits each and hash each band
					bands := 8
					rowsPerBand := len(signature) / bands
					for b := range bands {
						bandBits := signature[b*rowsPerBand : (b+1)*rowsPerBand]
						bandStr := boolSliceToString(bandBits)
						bucketKey := hashBand(bandStr)
						newBuckets[b] = bucketKey
					}

					for _, storedDesc := range user.Descriptor {
						match := isDescriptorMatch(storedDesc, newBuckets, 6)
						if match {
							if err := res.Emit(&user); err != nil {
								return err
							}
						}
					}
				}
			} else {
				if err := res.Emit(&q); err != nil {
					return err
				}
			}
		}

		return nil
	},
	Type: []byte{},
}

func stringToFloat64Slice(str string) ([]float64, error) {
	// Remove leading and trailing brackets
	str = strings.Trim(str, "[]")

	// Split the string into substrings representing numbers
	parts := strings.Split(str, ",")

	var floats []float64
	for _, part := range parts {
		// Trim whitespace from the part
		part = strings.TrimSpace(part)

		// Parse the part into a float64
		f, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, err
		}

		floats = append(floats, f)
	}

	return floats, nil
}

var OrbitDelDocsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Delete value by key",
		ShortDescription: `Key value store delete
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("dbName", true, false, "DB address or name"),
		cmds.StringArg("key", true, false, "Key to delete related value"),
	},
	PreRun: urlArgsEncoder,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		if err := urlArgsDecoder(req, env); err != nil {
			return err
		}

		dbName := req.Arguments[0]
		key := req.Arguments[1]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		if key == "all" {
			var records []map[string]interface{}
			_, err := store.Query(req.Context, func(e interface{}) (bool, error) {
				records = append(records, e.(map[string]interface{}))
				return true, nil
			})

			if err != nil {
				return err
			}

			for _, is := range records {
				for i := range is {
					if i == "_id" {
						id := fmt.Sprint(is[i])
						_, err := store.Delete(req.Context, id)
						if err != nil {
							return err
						}
					}
				}
			}
		} else {
			_, err := store.Delete(req.Context, key)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func ConnectKV(ctx context.Context, dbName string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.KeyValueStore, error) {
	datastore := filepath.Join(os.Getenv("HOME"), ".ipfs", "orbitdb")
	if _, err := os.Stat(datastore); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(datastore), 0755)
	}

	db, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &datastore,
	})
	if err != nil {
		return db, nil, err
	}

	var addr address.Address
	switch dbName {
	case dbNameDemandSupply:
		addr, err = db.DetermineAddress(ctx, dbName, "keyvalue", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameDemandSupply, "keyvalue", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	default:
		// return if the dbName is not expected
		return db, nil, errors.New("unexpected dbName")
	}

	store, err := db.KeyValue(ctx, addr.String(), &orbitdb.CreateDBOptions{})
	if err != nil {
		return db, nil, err
	}

	sub, err := db.EventBus().Subscribe(new(stores.EventReady))
	if err != nil {
		return db, nil, err
	}

	defer sub.Close()

	err = connectToPeers(api, ctx)
	if err != nil {
		return db, nil, err
	}

	go func() {
		for {
			for ev := range sub.Out() {
				switch ev.(type) {
				case *stores.EventReady:
					dbURI := store.Address().String()
					onReady(dbURI)
				}
			}
		}
	}()

	err = store.Load(ctx, -1)
	if err != nil {
		return db, nil, err
	}

	return db, store, nil
}

func ConnectDocs(ctx context.Context, dbName string, api iface.CoreAPI, onReady func(address string)) (orbitdb.OrbitDB, orbitdb.DocumentStore, error) {
	datastore := filepath.Join(os.Getenv("HOME"), ".ipfs", "orbitdb")
	if _, err := os.Stat(datastore); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(datastore), 0755)
	}

	db, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &datastore,
	})
	if err != nil {
		return db, nil, err
	}

	var addr address.Address
	switch dbName {
	case dbSubscription:
		addr, err = address.Parse("/orbitdb/bafyreidx3h677sge45q7phks6absvzffhtkdq2eikbcw44v5ah5aznxi2q/subscription")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbSubscription, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbPlan:
		addr, err = address.Parse("/orbitdb/bafyreif4qtyuyd257xtykgtxrkasczxtkltypadwhisu6d433lk4bwfww4/plan")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbPlan, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbInflation:
		addr, err = address.Parse("/orbitdb/bafyreia2equctbkzplsu4iwbhy7yagbddrdewxrpa3dejb3husompg5pde/inflation")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbInflation, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbTransaction:
		addr, err = address.Parse("/orbitdb/bafyreic2jvgdf3yegqxr6fecn7vlxw456xmn3kwzgajmygkmhlhhrqh54y/transaction")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbTransaction, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbIncome:
		addr, err = address.Parse("/orbitdb/bafyreiconechfko4uyw3ftnvwal3aen2xkcjgmbnnyxqappxr6g67xooxa/income")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbIncome, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbWallet:
		addr, err = address.Parse("/orbitdb/bafyreib6gs7jzfprajuxwg5wk6x33zueenah7h53tsks2tklusfx3iqz6q/wallet")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbWallet, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbUser:
		addr, err = address.Parse("/orbitdb/bafyreifr7e2axymbr2ufaxzgedoiy4oli24uy3zz7psoau4tglh7th6h7u/user")
		if err != nil {
			return db, nil, err
		}
		// addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		// if err != nil {
		// 	_, err = db.Create(ctx, dbUser, "docstore", &orbitdb.CreateDBOptions{})
		// 	if err != nil {
		// 		return db, nil, err
		// 	}
		// }
	case dbNameIssue:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameIssue, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameCitizenReputation:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameCitizenReputation, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameEvent:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameEvent, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameGift:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameGift, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbNameRide:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbNameRide, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	default:
		// return if the dbName is not expected
		return db, nil, errors.New("unexpected dbName")
	}

	// writeAccess := []string{db.Identity().ID}

	// ipfsAccessController := accesscontroller.NewManifestParams(cid.Cid{}, true, "ipfs")

	// ipfsAccessController.SetAccess("write", writeAccess)

	// authorized := ipfsAccessController.GetAllAccess()

	// logger.Println("my identity: ", db.Identity().ID)

	// logger.Println("authorized list: ", authorized)

	// opts := &orbitdb.CreateDBOptions{
	// 	AccessController: ipfsAccessController,
	// }

	store, err := db.Docs(ctx, addr.String(), &orbitdb_iface.CreateDBOptions{})
	if err != nil {
		return db, nil, err
	}

	sub, err := db.EventBus().Subscribe(new(stores.EventReady))
	if err != nil {
		return db, nil, err
	}

	defer sub.Close()

	err = connectToPeers(api, ctx)
	if err != nil {
		return db, nil, err
	}

	go func() {
		for {
			for ev := range sub.Out() {
				switch ev.(type) {
				case *stores.EventReady:
					dbURI := store.Address().String()
					onReady(dbURI)
				}
			}
		}
	}()

	err = store.Load(ctx, -1)
	if err != nil {
		return db, nil, err
	}

	return db, store, nil
}

func connectToPeers(c iface.CoreAPI, ctx context.Context) error {
	var wg sync.WaitGroup

	peerInfos, err := config.DefaultBootstrapPeers()
	if err != nil {
		return err
	}

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			err := c.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				fmt.Println("failed to connect", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
			} else {
				fmt.Println("connected!", zap.String("peerID", peerInfo.ID.String()))
			}
		}(&peerInfo)
	}
	wg.Wait()
	return nil
}

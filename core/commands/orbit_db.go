package commands

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	logger "log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	config "github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	iface "github.com/ipfs/kubo/core/coreiface"
	peer "github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
	"golang.org/x/crypto/pbkdf2"

	cmds "github.com/stateless-minds/go-ipfs-cmds"
	orbitdb "github.com/stateless-minds/go-orbit-db"
	"github.com/stateless-minds/go-orbit-db/address"
	orbitdb_iface "github.com/stateless-minds/go-orbit-db/iface"
	"github.com/stateless-minds/go-orbit-db/stores"
)

const dbNameDemandSupply = "demand_supply"
const dbNameCitizenReputation = "citizen_reputation"
const dbNameIssue = "issue"
const dbNameEvent = "event"
const dbNameGift = "gift"
const dbNameRide = "ride"
const dbUser = "user"
const dbIncome = "income"
const dbUserBalance = "user_balance"
const dbTransaction = "transaction"
const dbInflation = "inflation"
const dbPlan = "plan"
const transactionsPerPerson = 100

const (
	privateKeyFile      = "orbitdb_private.pem"   // Define a constant for the private key file name
	encryptedAESKeyFile = "encrypted_aes_key.bin" // File to store the encrypted AES key
	saltSize            = 16                      // Size of the random salt in bytes
)

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

// --- Key Management ---

// generatePrivateKey generates an RSA private key and saves it to a file.
func generatePrivateKey() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048) // You can adjust the key size
	if err != nil {
		return nil, err
	}

	// Convert private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Create the private key file
	file, err := os.Create(privateKeyFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Write the private key to the file
	err = pem.Encode(file, privateKeyBlock)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// loadPrivateKey loads an RSA private key from a file.
func loadPrivateKey() (*rsa.PrivateKey, error) {
	privateKeyFileBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}

	privateKeyBlock, _ := pem.Decode(privateKeyFileBytes)
	if privateKeyBlock == nil {
		return nil, err
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// getPrivateKey retrieves the private key, generating it if it doesn't exist.
func getPrivateKey() (*rsa.PrivateKey, error) {
	if _, err := os.Stat(privateKeyFile); os.IsNotExist(err) {
		return generatePrivateKey()
	}
	return loadPrivateKey()
}

// getAesKey retrieves the AES key, generating it if it doesn't exist.
func getAesKey() ([]byte, error) {
	if _, err := os.Stat(encryptedAESKeyFile); os.IsNotExist(err) {
		aesKey, err := deriveAESKey()
		if err != nil {
			return nil, err
		}

		// Encrypt the AES key with RSA for storage.
		privateKey, err := getPrivateKey()
		if err != nil {
			return nil, err
		}

		encryptedAESKeyBytes, err := rsa.EncryptPKCS1v15(rand.Reader, &privateKey.PublicKey, aesKey)
		if err != nil {
			return nil, err
		}

		// Store the encrypted AES key in a file.
		if err = os.WriteFile(encryptedAESKeyFile, encryptedAESKeyBytes, 0644); err != nil {
			return nil, err
		}

		return aesKey, nil // Return the newly generated AES key
	}

	// If the encrypted AES key file exists, read and decrypt it.
	data, err := ioutil.ReadFile(encryptedAESKeyFile)
	if err != nil {
		return nil, err
	}

	privateKey, err := getPrivateKey()
	if err != nil {
		return nil, err
	}

	aesKeyBytes, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, data)
	if err != nil {
		return nil, err
	}

	return aesKeyBytes, nil // Return the decrypted AES key
}

// --- Password Management ---

// deriveAESKey derives an AES key from a password using PBKDF2.
func deriveAESKey() ([]byte, error) {
	password := os.Getenv("ENC_PASSWORD") // Retrieve the password from the environment variable
	if password == "" {
		return nil, errors.New("no password found in environment variable")
	}
	salt, err := generateRandomSalt(saltSize)
	if err != nil {
		return nil, err
	}
	return pbkdf2.Key([]byte(password), []byte(salt), 10000, 32, sha256.New), nil // 32 bytes for AES-256
}

// generateRandomSalt generates a random salt of specified size.
func generateRandomSalt(size int) ([]byte, error) {
	salt := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// --- Encryption/Decryption ---

// aesEncrypt encrypts data using AES.
func aesEncrypt(data []byte, aesKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// aesDecrypt decrypts data using AES.
func aesDecrypt(ciphertext []byte, aesKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, io.ErrUnexpectedEOF
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	decryptedData, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decryptedData, nil
}

// encryptData encrypts data using AES and returns encrypted data while storing encrypted AES key.
func encryptData(data []byte) (string, error) {
	aesKey, err := getAesKey()
	if err != nil {
		return "", err
	}

	encryptedData, err := aesEncrypt(data, aesKey)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encryptedData), nil
}

// decryptData decrypts the encrypted data using the user's password.
func decryptData(encryptedData string) ([]byte, error) {
	// Decode base64 strings.
	encryptedDataBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, err
	}

	// Derive AES key from password (you may also use a salt here).
	derivedAESKey, err := getAesKey()
	if err != nil {
		return nil, err
	}

	// Decrypt data with AES using the derived AES key.
	data, err := aesDecrypt(encryptedDataBytes, derivedAESKey)
	if err != nil {
		return nil, err
	}

	return data, nil
}

var OrbitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental orbit-db integration on ipfs.",
		ShortDescription: `
orbit db is a p2p database on top of ipfs node
`,
	},
	Subcommands: map[string]*cmds.Command{
		"kvput":        OrbitPutKVCmd,
		"kvget":        OrbitGetKVCmd,
		"kvdel":        OrbitDelKVCmd,
		"docsput":      OrbitPutDocsCmd,
		"docsputenc":   OrbitPutDocsEncryptedCmd,
		"docsget":      OrbitGetDocsCmd,
		"docsquery":    OrbitQueryDocsCmd,
		"docsqueryenc": OrbitQueryDocsEncryptedCmd,
		"docsdel":      OrbitDelDocsCmd,
		"runindexer":   OrbitIndexerCmd,
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

		data, err := ioutil.ReadAll(file)
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

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		dd := make(map[string]interface{})
		if err := json.Unmarshal(data, &dd); err != nil {
			panic(err)
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

var OrbitPutDocsEncryptedCmd = &cmds.Command{
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

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		dd := make(map[string]interface{})
		if err := json.Unmarshal(data, &dd); err != nil {
			panic(err)
		}

		for key, val := range dd {
			if key != "_id" {
				valJSON, err := json.Marshal(val)
				if err != nil {
					panic(err)
				}

				encryptedData, err := encryptData(valJSON)
				if err != nil {
					panic(err)
				}

				dd[key] = encryptedData
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

			if err := res.Emit(&val[0]); err != nil {
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

var OrbitIndexerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Adjust basic income based on inflation/deflation",
		ShortDescription: `Inflation indexer`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		timeStart := time.Now()
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			logger.Println("cmdenv.GetApi error: ", err)
			return err
		}

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

			dbInc.Close()

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

				for _, in := range incomes {
					if in.Period == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month()+1)) {
						logger.Println("income for next month exists, quitting")
						return errors.New("income for next month already exists")
					} else if in.Period == strconv.Itoa(time.Now().Year())+"/"+strconv.Itoa(int(time.Now().Month())) {
						currentIncome = in.Amount
					}
				}

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

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			record := e.(map[string]interface{})
			if key == "all" {
				return true, nil
			} else if strings.Contains(value, ",") {
				values := strings.Split(value, ",")
				recs, ok := record[key].(string)
				if !ok {
					return false, nil
				}
				if strings.Contains(recs, ",") {
					records := strings.Split(recs, ",")
					for _, r := range records {
						for _, v := range values {
							if r == v {
								return true, nil
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
			panic(err)
		}

		dataString := string(dataBytes)

		if dataString == "null" {
			if err := res.Emit(nil); err != nil {
				return err
			}
		} else {
			if err := res.Emit(&q); err != nil {
				return err
			}
		}

		return nil
	},
	Type: []byte{},
}

var OrbitQueryDocsEncryptedCmd = &cmds.Command{
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
		// value := req.Arguments[2]

		db, store, err := ConnectDocs(req.Context, dbName, api, func(address string) {})
		if err != nil {
			return err
		}

		defer db.Close()

		q, err := store.Query(req.Context, func(e interface{}) (bool, error) {
			if key == "all" {
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
			panic(err)
		}

		dataString := string(dataBytes)

		if dataString == "null" {
			if err := res.Emit(nil); err != nil {
				return err
			}
		} else {
			var dd []map[string]string

			if err := json.Unmarshal(dataBytes, &dd); err != nil {
				panic(err)
			}

			for _, item := range dd {
				for key, val := range item {
					if key != "_id" {
						decryptedData, err := decryptData(val)
						if err != nil {
							return err
						}

						item[key] = string(decryptedData)
					}
				}
			}

			if err := res.Emit(&dd); err != nil {
				return err
			}
		}

		return nil
	},
	Type: []byte{},
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

	store.Load(ctx, -1)
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
	case dbPlan:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbPlan, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbInflation:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbInflation, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbTransaction:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbTransaction, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbIncome:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbIncome, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbUserBalance:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbUserBalance, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
	case dbUser:
		addr, err = db.DetermineAddress(ctx, dbName, "docstore", &orbitdb_iface.DetermineAddressOptions{})
		if err != nil {
			_, err = db.Create(ctx, dbUser, "docstore", &orbitdb.CreateDBOptions{})
			if err != nil {
				return db, nil, err
			}
		}
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

	store, err := db.Docs(ctx, addr.String(), &orbitdb.CreateDBOptions{})
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

	store.Load(ctx, -1)
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

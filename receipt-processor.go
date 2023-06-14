package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"math"
	"unicode"
)

type Receipt struct {
	Retailer     string    `json:"retailer"`
	PurchaseDate string    `json:"purchaseDate"`
	PurchaseTime string    `json:"purchaseTime"`
	Items        []Item    `json:"items"`
	Total        string    `json:"total"`
	Points       int       `json:"points,omitempty"`
}

type Item struct {
	ShortDescription string `json:"shortDescription"`
	Price            string `json:"price"`
}

type PointsResponse struct {
	Points int `json:"points"`
}

var (
	mutex    sync.Mutex
	receipts map[string]Receipt
)

// Create receipts map as our in-memory database.
// Configure our two endpoints.
func main() {
	receipts = make(map[string]Receipt)

	http.HandleFunc("/receipts/process", ProcessReceipt)
	http.HandleFunc("/receipts/", GetPointsHandler)

	log.Println("Server started on port 8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

// Helper to check for /points endpoint of /receipts
func GetPointsHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/points") {
		GetPoints(w, r)
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// Process a receipt by reading in JSON data and calculating
// the points. Save the info in the receipts map.
func ProcessReceipt(w http.ResponseWriter, r *http.Request) {
	var receipt Receipt

	// If the receipt is invalid, return 400
	err := json.NewDecoder(r.Body).Decode(&receipt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Protect receipts from concurrent modification
	mutex.Lock()
	defer mutex.Unlock()

	calculatePoints(&receipt)

	id, err := generateID()
	// If there was an error generating the ID, return it
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	receipts[id] = receipt

	response := struct {
		ID string `json:"id"`
	}{
		ID: id,
	}

	jsonResponse(w, response)
}

// Generate a random ID
func generateID() (string, error) {
	// Generate 16 random bytes
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Convert the bytes to a hexadecimal string
	id := hex.EncodeToString(bytes)

	return id, nil
}

// Return the points of a given receipt
func GetPoints(w http.ResponseWriter, r *http.Request) {
	id := strings.Split(r.URL.Path, "/")[2]

	mutex.Lock()
	defer mutex.Unlock()

	receipt, ok := receipts[id]
	if !ok {
		http.Error(w, "Invalid receipt ID", http.StatusBadRequest)
		return
	}

	response := PointsResponse{Points: receipt.Points}
	jsonResponse(w, response)
}

// Calculate the points of a receipt based on seven rules.
func calculatePoints(receipt *Receipt) {
	points := 0

	// Rule 1: One point for every alphanumeric character in the retailer name.
	retailer := receipt.Retailer
	alphanumericRetailer := ""
	for _, char := range retailer {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			alphanumericRetailer += string(char)
		}
	}
	points += len(alphanumericRetailer)

	// Rule 2: 50 points if the total is a round dollar amount with no cents.
	total, _ := strconv.ParseFloat(receipt.Total, 64)
	if total == float64(int(total)) {
		points += 50
	}

	// Rule 3: 25 points if the total is a multiple of 0.25.
	if int(total*100)%25 == 0 {
		points += 25
	}

	// Rule 4: 5 points for every two items on the receipt.
	itemCount := len(receipt.Items)
	pairCount := itemCount / 2
	points += pairCount * 5

	// Rule 5: If the trimmed length of the item description is a multiple of 3,
	// multiply the price by 0.2 and round up to the nearest integer. The result is the number of points earned.
	for _, item := range receipt.Items {
		trimmedLength := len(strings.TrimSpace(item.ShortDescription))
		if trimmedLength%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			itemPoints := int(math.Ceil(price * 0.2))
			if itemPoints > 0 {
				points += itemPoints
			}
		}
	}

	// Rule 6: 6 points if the day in the purchase date is odd.
	purchaseDate, _ := time.Parse("2006-01-02", receipt.PurchaseDate)
	if purchaseDate.Day()%2 != 0 {
		points += 6
	}

	// Rule 7: 10 points if the time of purchase is after 2:00pm and before 4:00pm.
	purchaseTime, _ := time.Parse("15:04", receipt.PurchaseTime)
	if purchaseTime.After(time.Date(purchaseTime.Year(), purchaseTime.Month(), purchaseTime.Day(), 14, 0, 0, 0, purchaseTime.Location())) &&
		purchaseTime.Before(time.Date(purchaseTime.Year(), purchaseTime.Month(), purchaseTime.Day(), 16, 0, 0, 0, purchaseTime.Location())) {
		points += 10
	}

	receipt.Points = points
}

// Utility function to send JSON-encoded responses in HTTP
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

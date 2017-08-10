package chapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/WedgeNix/util"
)

const (
	// rate is the rate at which we can call the CA api.
	rate = time.Minute / 1999
)

// settings for debugging alex made the real struct
// and I will add these tow the settings file
type settings struct {
	Filters   string
	Expand    string
	Translate interface{}
}

// AttributeValue CA api Data
type AttributeValue struct {
	Name      string
	ProductID int
	ProfileID int
	Value     string
}

// Product CA api Data
type Product struct {
	ID                            int
	ProfileID                     int
	CreateDateUtc                 time.Time
	IsInRelationship              bool
	IsParent                      bool
	RelationshipName              string
	ParentProductID               int
	IsAvailableInStore            bool
	IsBlocked                     bool
	IsExternalQuantityBlocked     bool
	BlockComment                  string
	BlockedDateUtc                time.Time
	ReceivedDateUtc               time.Time
	LastSaleDateUtc               time.Time
	UpdateDateUtc                 time.Time
	QuantityUpdateDateUtc         time.Time
	ASIN                          string
	Brand                         string
	Condition                     string
	Description                   string
	EAN                           string
	FlagDescription               string
	Flag                          string
	HarmonizedCode                string
	ISBN                          string
	Manufacturer                  string
	MPN                           string
	ShortDescription              string
	Sku                           string
	Subtitle                      string
	TaxProductCode                string
	Title                         string
	UPC                           string
	WarehouseLocation             string
	Warranty                      string
	Height                        float64
	Length                        float64
	Width                         float64
	Weight                        float64
	Cost                          float64
	Margin                        float64
	RetailPrice                   float64
	StartingPrice                 float64
	ReservePrice                  float64
	BuyItNowPrice                 float64
	StorePrice                    float64
	SecondChancePrice             float64
	SupplierName                  string
	SupplierCode                  string
	SupplierPO                    string
	Classification                string
	IsDisplayInStore              bool
	StoreTitle                    string
	StoreDescription              string
	BundleType                    string
	TotalAvailableQuantity        int
	OpenAllocatedQuantity         int64
	OpenAllocatedQuantityPooled   int64
	PendingCheckoutQuantity       int64
	PendingCheckoutQuantityPooled int64
	PendingPaymentQuantity        int64
	PendingPaymentQuantityPooled  int64
	PendingShipmentQuantity       int64
	PendingShipmentQuantityPooled int64
	TotalQuantity                 int64
	TotalQuantityPooled           int64
	Attributes                    []AttributeValue
	DCQuantities                  []DCQuantity
	Images                        []Image
	Labels                        []ProductLabel
	BundleComponents              []ProductBundleComponent
	Children                      []ChildRelationship
}

// DCQuantity structs for CA api data
type DCQuantity struct {
	ProductID            int
	ProfileID            int
	DistributionCenterID int
	AvailableQuantity    int
	Product              interface{}
}

// Image structs for CA api data
type Image struct {
	Abbreviation  string
	PlacementName string
	ProductID     int
	ProfileID     int
	URL           string
}

// ProductLabel structs for CA api data
type ProductLabel struct {
	ProductID int
	ProfileID int
	Name      string
	Product   interface{}
}

// ProductBundleComponent structs for CA api data
type ProductBundleComponent struct {
	ProductID    int
	ComponentID  int
	ProfileID    int
	ComponentSku string
	Quantity     int
	Product      interface{}
}

// ChildRelationship structs for CA api data
type ChildRelationship struct {
	ParentProductID int
	ProfileID       int
	ChildProductID  int
	ChildProduct    interface{}
}

// chaData top level structure of data given form CA server
type chaData struct {
	Context  string    `json:"@odata.context"`
	Value    []Product `json:"value"`
	NextLink string    `json:"@odata.nextLink"`
}

// CaObj is a struct that has the things needed for a call to CA.
type CaObj struct {
	client   *http.Client
	isParent bool
}

// New creates object for api call to Channel Advisor
func New() (*CaObj, error) {
	ctx := context.Background()
	tok, conf, err := initCaAuth(ctx)
	if err != nil {
		return nil, err
	}
	client := conf.Client(ctx, tok)
	return &CaObj{
		client:   client,
		isParent: true,
	}, nil
}

// Parent can change the call to childeren only by puting false in Parent.
func (ca *CaObj) Parent(ip bool) {
	ca.isParent = ip
}

// GetCAData is main function for this package it calles Channel advisor for data
func (ca *CaObj) GetCAData() <-chan []Product {
	settings := settings{}
	util.Load("channel_settings", &settings)
	tick := time.Tick(rate)

	var fullRes []Product
	link := ""
	end := make(chan bool)
	fullResch := make(chan []Product)
	go func() {
		<-end
		util.Log("Return")
		fullResch <- fullRes
	}()
	for {
		<-tick
		data := chaData{}
		if link == "" {
			vals := url.Values(make(map[string][]string))
			filter := fmt.Sprintf("%s AND IsParent eq %v", settings.Filters, ca.isParent)
			vals.Set("$filter", filter)
			vals.Set("$expand", settings.Expand)
			link = "https://api.channeladvisor.com/v1/Products?" + vals.Encode()
		}
		util.Log("Starting call")
		util.Log(link)
		res, err := ca.client.Get(link)
		util.E(err)
		util.Log("End of call")

		defer res.Body.Close()
		json.NewDecoder(res.Body).Decode(&data)
		util.E(err)

		if len(fullRes) > 0 {
			fullRes = append(fullRes, data.Value...)
		} else {
			fullRes = data.Value
		}
		link = data.NextLink
		util.Log(link)

		if link == "" {
			util.Log("ending")
			end <- true
			break
		}

	}
	return fullResch
}

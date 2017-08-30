package chapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
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
		client: client,
	}, nil
}

// Parent can change the call to childeren only by puting false in Parent.
func (ca *CaObj) Parent(ip bool) {
	ca.isParent = ip
}

// GetCAData is main function for this package it calles Channel advisor for data.
func (ca *CaObj) GetCAData(date time.Time) ([]Product, error) {
	tick := time.Tick(rate / 5)
	prods := []Product{}
	prodsLock := sync.Mutex{}
	skip := 0

	done := make(chan bool)
	wait := make(chan bool)
	workers := make(chan int, 5)
	workers <- 1
	workers <- 2
	workers <- 3
	workers <- 4
	workers <- 5

	working := sync.WaitGroup{}

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				wait <- true
			}
		}
	}()

	for id := range workers {
		<-tick

		working.Add(1)
		go func(id int, skip int, date time.Time) {

			vals := url.Values{}
			filter := "Labels/Any (c: c/Name eq 'Foreign Accounts') AND TotalAvailableQuantity gt 0 AND ProfileID eq 32001166"
			if ca.isParent {
				filter += "AND IsParent eq true"
			}
			filter += "AND CreateDateUtc ge " + date.Format("2006-01-02")
			vals.Set("$filter", filter)
			vals.Set("$expand", "Attributes,Labels,Images")
			vals.Set("$skip", strconv.Itoa(skip))
			link := "https://api.channeladvisor.com/v1/Products?" + vals.Encode()
			fmt.Print(`[skip`, skip, `]`)

			resp, err := ca.client.Get(link)
			if err != nil {
				log.Fatalln(err)
			}

			data := &chaData{}
			err = json.NewDecoder(resp.Body).Decode(data)
			if err != nil {
				log.Fatalln(err)
			}
			defer resp.Body.Close()

			prodsLock.Lock()
			prods = append(prods, data.Value...)
			prodsLock.Unlock()

			working.Done()

			<-wait
			if len(data.NextLink) > 0 {
				workers <- id
			} else {
				done <- true
				close(workers)
			}
		}(id, skip, date)

		skip += 100
	}
	working.Wait()

	return prods, nil
}

func (ca CaObj) save(r io.Reader, region int) error {
	if region == 0 {
		return errors.New("region not set")
	}

	req, err := http.NewRequest(
		"POST",
		`https://api.channeladvisor.com/v1/ProductUpload?profileid=`+strconv.Itoa(region),
		r,
	)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "text/csv")
	resp, err := ca.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode > 299 {
		return errors.New(resp.Status)
	}

	return nil
}

func commaSepProds(prods []Product) (csv [][]string) {
	csv = append(csv, []string{
		"Auction Title",
		// "Inventory Number",
		// "Item Create Date",
		// "Height",
		// "Length",
		// "Width",
		// "Weight",
		// "UPC",
		// "Description",
		// "Brand",
		// "Condition",
		// "Seller Cost",
		// "Buy It Now Price",
		// "Picture URLs",
		// "Received In Inventory",
		// "Relationship Name",
		// "Variation Parent SKU",
		// "Labels",
		// "Classification",
		// "Attribute1Name",
		// "Attribute1Value",
		// "Attribute2Name",
		// "Attribute2Value",
		// "Attribute3Name",
		// "Attribute3Value",
		// "Attribute4Name",
		// "Attribute4Value",
		// "Attribute5Name",
		// "Attribute5Value",
		// "Attribute6Name",
		// "Attribute6Value",
		// "Attribute7Name",
		// "Attribute7Value",
		// "Attribute5Value",
		// "Attribute9Name",
		// "Attribute9Value",
		// "Attribute10Name",
		// "Attribute10Value",
		// "Attribute11Name",
		// "Attribute11Value",
		// "Attribute12Name",
		// "Attribute12Value",
		// "Attribute13Name",
		// "Attribute13Value",
		// "Attribute14Name",
		// "Attribute14Value",
		// "Attribute15Name",
		// "Attribute15Value",
		// "Attribute16Name",
		// "Attribute16Value",
		// "Attribute17Name",
		// "Attribute17Value",
		// "Attribute18Name",
		// "Attribute18Value",
		// "Attribute19Name",
		// "Attribute19Value",
		// "Attribute20Name",
		// "Attribute20Value",
		// "Attribute21Name",
		// "Attribute21Value",
		// "Attribute22Name",
		// "Attribute22Value",
		// "Attribute23Name",
		// "Attribute23Value",
		// "Attribute24Name",
		// "Attribute24Value",
		// "Attribute25Name",
		// "Attribute25Value",
		// "Attribute26Name",
		// "Attribute26Value",
		// "Attribute27Name",
		// "Attribute27Value",
	})

	for _, prod := range prods {
		csv = append(csv, []string{
			prod.Title,
			// 	prod.InventoryNumber,
			// 	prod.ItemCreateDate,
			// 	prod.Height,
			// 	prod.Length,
			// 	prod.Width,
			// 	prod.Weight,
			// 	prod.UPC,
			// 	prod.Description,
			// 	prod.Brand,
			// 	prod.Condition,
			// 	prod.SellerCost,
			// 	prod.BuyItNowPrice,
			// 	prod.PictureURLs,
			// 	prod.ReceivedInInventory,
			// 	prod.RelationshipName,
			// 	prod.VariationParentSKU,
			// 	prod.Labels,
			// 	prod.Classification,
			// 	prod.Attribute1Name,
			// 	prod.Attribute1Value,
			// 	prod.Attribute2Name,
			// 	prod.Attribute2Value,
			// 	prod.Attribute3Name,
			// 	prod.Attribute3Value,
			// 	prod.Attribute4Name,
			// 	prod.Attribute4Value,
			// 	prod.Attribute5Name,
			// 	prod.Attribute5Value,
			// 	prod.Attribute6Name,
			// 	prod.Attribute6Value,
			// 	prod.Attribute7Name,
			// 	prod.Attribute7Value,
			// 	prod.Attribute8Name,
			// 	prod.Attribute8Value,
			// 	prod.Attribute9Name,
			// 	prod.Attribute9Value,
			// 	prod.Attribute10Name,
			// 	prod.Attribute10Value,
			// 	prod.Attribute11Name,
			// 	prod.Attribute11Value,
			// 	prod.Attribute12Name,
			// 	prod.Attribute12Value,
			// 	prod.Attribute13Name,
			// 	prod.Attribute13Value,
			// 	prod.Attribute14Name,
			// 	prod.Attribute14Value,
			// 	prod.Attribute15Name,
			// 	prod.Attribute15Value,
			// 	prod.Attribute16Name,
			// 	prod.Attribute16Value,
			// 	prod.Attribute17Name,
			// 	prod.Attribute17Value,
			// 	prod.Attribute18Name,
			// 	prod.Attribute18Value,
			// 	prod.Attribute19Name,
			// 	prod.Attribute19Value,
			// 	prod.Attribute20Name,
			// 	prod.Attribute20Value,
			// 	prod.Attribute21Name,
			// 	prod.Attribute21Value,
			// 	prod.Attribute22Name,
			// 	prod.Attribute22Value,
			// 	prod.Attribute23Name,
			// 	prod.Attribute23Value,
			// 	prod.Attribute24Name,
			// 	prod.Attribute24Value,
			// 	prod.Attribute25Name,
			// 	prod.Attribute25Value,
			// 	prod.Attribute26Name,
			// 	prod.Attribute26Value,
			// 	prod.Attribute27Name,
			// 	prod.Attribute27Value,
		})
	}

	return
}

// SendBinaryCSV turns products into a binary CSV.
func (ca CaObj) SendBinaryCSV(csvLayout [][]string, region int) error {
	buf := new(bytes.Buffer)

	w := csv.NewWriter(buf)
	err := w.WriteAll(csvLayout)
	if err != nil {
		panic(err)
	}
	w.Flush()

	b := buf.Bytes()
	buf.Reset()

	err = binary.Write(buf, binary.BigEndian, b)
	if err != nil {
		panic(err)
	}

	return ca.save(buf, region)
}

package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"

	"github.com/chromato99/WiFi_Positioning_Server_go/result"
	"golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

// struct for json data from client
type PosData struct {
	Position string     `json:"position"`
	Password string     `json:"password"`
	WifiData []WifiData `json:"wifi_data"`
}

// wifi data in PosData
type WifiData struct {
	Bssid string `json:"bssid"`
	Rssi  int    `json:"rssi"`
}

// data from DB
type DBData struct {
	Id       int
	Position string
	WifiData []WifiData
}

// password data struct
type Passwd struct {
	Key string `json:"key"`
}

/*
Parameter : c - Gin web framework context
Return value : db - DB opened sql.DB struct.

Feature - Open the DB with env value and return sql.DB struct
*/
func openDB(c *gin.Context) (*sql.DB, error) {

	// db settings
	cfg := mysql.Config{
		DBName: os.Getenv("MYSQL_DB"),
		Addr:   os.Getenv("MYSQL_HOST"),
		User:   os.Getenv("MYSQL_USER"),
		Passwd: os.Getenv("MYSQL_PASSWORD"),
		Net:    "tcp",
	}

	// open mysql db
	db, dbError := sql.Open("mysql", cfg.FormatDSN())
	if dbError != nil {
		return nil, dbError
	}

	return db, nil
}

// Returns the received Wi-Fi data for the test.
func Test(c *gin.Context) {
	rawData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "GetRawData error!!",
		})
		return
	}
	var data PosData
	json.Unmarshal([]byte(rawData), &data)
	doc, _ := json.Marshal(data.WifiData)

	c.IndentedJSON(http.StatusOK, string(doc))
}

// Add wifi location data to database
func AddData(c *gin.Context) {

	// Process the received json data
	rawData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "GetRawData error!!",
		})
		return
	}
	var newData PosData
	json.Unmarshal([]byte(rawData), &newData)
	fmt.Println(newData)
	wifiData, _ := json.Marshal(newData.WifiData)

	// Reading hashed password file
	passwordFile, err := os.Open("./core/password.json")
	var bcryptErr error

	if err != nil {
		// When password is not set
		bcryptErr = nil
	} else {
		passwordByte, err := ioutil.ReadAll(passwordFile)
		if err != nil {
			//Handle Error
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "Read PW file error!!",
			})
			return
		}

		var pw Passwd
		// Convert json format to golang format
		json.Unmarshal(passwordByte, &pw)

		// compare password with sotred password
		bcryptErr = bcrypt.CompareHashAndPassword([]byte(pw.Key), []byte(newData.Password))
	}

	if bcryptErr == nil {
		db, err := openDB(c)
		if err != nil {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB open error!!",
			})
			return
		}

		// insert wifi position data
		result, err := db.Exec("INSERT INTO wifi_data (position, wifi_data) VALUES (?, ?)", newData.Position, string(wifiData))
		if err != nil {
			fmt.Println(err)
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB insert error!!",
			})
			return
		}

		db.Close()

		id, err := result.LastInsertId()
		if err != nil {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "Get LastInsertId error!!",
			})
			return
		}

		// when insert data sucessed
		c.IndentedJSON(http.StatusOK, gin.H{
			"status":     "success",
			"insertedId": id,
		})
	} else {
		c.IndentedJSON(http.StatusOK, gin.H{
			"status": "password invalid",
		})
	}

}

// Function for estimates your current location using new Wi-Fi signal data.
func FindPosition(c *gin.Context) {
	// Process the received json data
	rawInputData, err := c.GetRawData()
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "GetRawData error!!",
		})
		return
	}
	var inputData PosData
	json.Unmarshal([]byte(rawInputData), &inputData)

	db, err := openDB(c)
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "DB open error!!",
		})
		return
	}

	// get all wifi position data from db
	rows, err := db.Query("SELECT * FROM wifi_data")
	if err != nil {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "DB select query error!!",
		})
		return
	}

	db.Close()

	// Append received db data to array
	var dbDataList []DBData
	for rows.Next() {
		var dbData DBData
		var rawWifiData string
		if err := rows.Scan(&dbData.Id, &dbData.Position, &rawWifiData); err != nil {
			fmt.Printf("DB Scan Error : %v", err)
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB Scan Error!!",
			})
			return
		}
		json.Unmarshal([]byte(rawWifiData), &dbData.WifiData)
		dbDataList = append(dbDataList, dbData)
	}

	rows.Close()

	// The part that calls the position estimation operation
	ch := make(chan []*result.ResultData)
	var results []*result.ResultData

	// Create 3 threads only when db_pos_arr is greater than 3
	if len(dbDataList) > 3 {
		threadNum, err := strconv.Atoi(os.Getenv("THREAD_NUM"))
		if err != nil {
			fmt.Println(err)
		}
		sliceLen := int(math.Ceil(float64(len(dbDataList)) / float64(threadNum-1)))

		for i := 0; i < threadNum-2; i++ {
			go calcPos(dbDataList[sliceLen*i:sliceLen*(i+1)], inputData, 0.6, ch)
		}
		go calcPos(dbDataList[sliceLen*2:], inputData, 0.6, ch)

		for i := 0; i < threadNum-1; i++ {
			results = append(results, <-ch...)
		}
	} else {
		go calcPos(dbDataList, inputData, 0.6, ch)
		results = append(results, <-ch...)
	}

	bestResult := calcKnn(results, 4)

	// respond best position result
	c.IndentedJSON(http.StatusOK, gin.H{
		"Position": bestResult.Position,
		"k_count":  bestResult.Count,
	})
}

/*
Parameter dbDataList : Position and wifi data stored in database.
Parameter inputData : Requested wifi data to find position.
Parameter margin : The range of data to compare from the data with the most overlapping bssids (because the more bssids are the same, the higher the accuracy)
Parameter ch : The channel to pass the return value as the function is called as a goroutine.
Return (by chan) topResults : Array of result values that are most similar and within the margin range when similarity is calculated.

Feature : It calculates the similarity between input data and db data and returns a list of top results within the margin range.
*/
func calcPos(dbDataList []DBData, inputData PosData, margin float64, ch chan []*result.ResultData) {
	var resultList result.ResultList

	for _, pos := range dbDataList {
		result := &result.ResultData{Id: pos.Id, Position: pos.Position, Count: 0, Avg: 0, Ratio: 0}
		var sum int = 0

		for _, wifi_data := range pos.WifiData {
			for _, input_wifi := range inputData.WifiData {
				if wifi_data.Bssid == input_wifi.Bssid {
					result.Count++
					sum += int(math.Abs(float64(wifi_data.Rssi) - float64(input_wifi.Rssi)))
					break
				}
			}
		}

		// calculate result average and ratio
		result.Avg = float64(sum) / float64(result.Count)
		result.Ratio = result.Avg / float64(result.Count)
		resultList.Push(result)
	}

	var largestCount int = 0
	if len(resultList) > 0 {
		largestCount = resultList[0].Count
	}
	var topResults []*result.ResultData
	for i, result_len := 0, len(resultList); i < result_len; i++ {
		result_data := resultList.Pop().(*result.ResultData)
		if float64(result_data.Count) <= (float64(largestCount) * margin) {
			break
		}
		topResults = append(topResults, result_data)
	}

	sort.Slice(topResults, func(i, j int) bool {
		return topResults[i].Ratio < topResults[j].Ratio
	})

	ch <- topResults
}

/*
Parameter results : List of result values calculated in calcPos.
Parameter k : The value of how many top values to compare.
Return value bestResult : The most likely result of the current user's location.

Feature : Among the results returned by calcPos, it compares k number of top results and returns the position result that matches the most.
*/
func calcKnn(results []*result.ResultData, k int) *result.ResultData {
	// Storing the number of same results in kCount map
	kCount := make(map[string]int)
	for i := 0; i < k && i < len(results); i++ {
		kCount[results[i].Position] += 1
	}

	// Init bestResult struct
	bestResult := &result.ResultData{
		Id:       0,
		Position: "not found",
		Count:    0,
		Avg:      0,
		Ratio:    0,
	}

	// Find the position with the highest value in kCount
	for key, value := range kCount {
		if value > bestResult.Count {
			bestResult.Count = value
			bestResult.Position = key
		} else if value == bestResult.Count && key == results[0].Position {
			bestResult.Count = value
			bestResult.Position = key
		}
	}

	// Print log
	if k < len(kCount) {
		fmt.Println(results[:k])
	} else {
		fmt.Println(results)
	}

	return bestResult
}

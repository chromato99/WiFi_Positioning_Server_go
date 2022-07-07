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

func OpenDB(c *gin.Context) (*sql.DB, error) {

	// db settings
	cfg := mysql.Config{
		DBName: os.Getenv("MYSQL_DB"),
		Addr:   os.Getenv("MYSQL_HOST"),
		User:   os.Getenv("MYSQL_USER"),
		Passwd: os.Getenv("MYSQL_PASSWORD"),
		Net:    "tcp",
	}

	// open mysql db
	db, dberror := sql.Open("mysql", cfg.FormatDSN())
	if dberror != nil {
		return nil, dberror
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
	wifi_data, _ := json.Marshal(newData.WifiData)

	// Reading hashed password file
	password_file, err := os.Open("./core/password.json")
	var bcrypterr error

	if err != nil {
		// When password is not set
		bcrypterr = nil
	} else {
		password_byte, err := ioutil.ReadAll(password_file)
		if err != nil {
			//Handle Error
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "Read PW file error!!",
			})
			return
		}

		var pw Passwd
		// Convert json format to golang format
		json.Unmarshal(password_byte, &pw)

		// compare password with sotred password
		bcrypterr = bcrypt.CompareHashAndPassword([]byte(pw.Key), []byte(newData.Password))
	}

	if bcrypterr == nil {
		db, err := OpenDB(c)
		if err != nil {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB open error!!",
			})
			return
		}

		// insert wifi position data
		result, err := db.Exec("INSERT INTO wifi_data (position, wifi_data) VALUES (?, ?)", newData.Position, string(wifi_data))
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

	db, err := OpenDB(c)
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
	var db_pos_arr []DBData
	for rows.Next() {
		var db_pos DBData
		var raw_wifi_data string
		if err := rows.Scan(&db_pos.Id, &db_pos.Position, &raw_wifi_data); err != nil {
			fmt.Printf("DB Scan Error : %v", err)
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": "DB Scan Error!!",
			})
			return
		}
		json.Unmarshal([]byte(raw_wifi_data), &db_pos.WifiData)
		db_pos_arr = append(db_pos_arr, db_pos)
	}

	rows.Close()

	// The part that calls the position estimation operation
	ch := make(chan []*result.ResultData)
	var result_arr []*result.ResultData

	// Create 3 threads only when db_pos_arr is greater than 3
	if len(db_pos_arr) > 3 {
		slice_len := int(math.Ceil(float64(len(db_pos_arr)) / 3))
		for i := 0; i < 2; i++ {
			go CalcPos(db_pos_arr[slice_len*i:slice_len*(i+1)], newData, 0.6, ch)
		}
		go CalcPos(db_pos_arr[slice_len*2:], newData, 0.6, ch)
		for i := 0; i < 3; i++ {
			result_arr = append(result_arr, <-ch...)
		}
	} else {
		go CalcPos(db_pos_arr, newData, 0.6, ch)
		result_arr = append(result_arr, <-ch...)
	}

	best_result := CalcKnn(result_arr, 4)

	// respond best position result
	c.IndentedJSON(http.StatusOK, gin.H{
		"Position": best_result.Position,
		"k_count":  best_result.Count,
	})
}

func CalcPos(DBPos []DBData, inputPos PosData, margin float64, ch chan []*result.ResultData) {
	var result_list result.ResultList

	for _, pos := range DBPos {
		result := &result.ResultData{Id: pos.Id, Position: pos.Position, Count: 0, Avg: 0, Ratio: 0}
		var sum int = 0

		for _, wifi_data := range pos.WifiData {
			for _, input_wifi := range inputPos.WifiData {
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
		result_list.Push(result)
	}

	var largest_count int = 0
	if len(result_list) > 0 {
		largest_count = result_list[0].Count
	}
	var top_result_arr []*result.ResultData
	for i, result_len := 0, len(result_list); i < result_len; i++ {
		result_data := result_list.Pop().(*result.ResultData)
		if float64(result_data.Count) <= (float64(largest_count) * margin) {
			break
		}
		top_result_arr = append(top_result_arr, result_data)
	}

	sort.Slice(top_result_arr, func(i, j int) bool {
		return top_result_arr[i].Ratio < top_result_arr[j].Ratio
	})

	ch <- top_result_arr
}

func CalcKnn(result_arr []*result.ResultData, k int) *result.ResultData {
	k_count := make(map[string]int)
	for i := 0; i < k && i < len(result_arr); i++ {
		k_count[result_arr[i].Position] += 1
	}

	best_result := &result.ResultData{
		Id:       0,
		Position: "not found",
		Count:    0,
		Avg:      0,
		Ratio:    0,
	}

	for key, value := range k_count {
		if value > best_result.Count {
			best_result.Count = value
			best_result.Position = key
		} else if value == best_result.Count && key == result_arr[0].Position {
			best_result.Count = value
			best_result.Position = key
		}
	}

	if k < len(k_count) {
		fmt.Println(result_arr[:k])
	} else {
		fmt.Println(result_arr)
	}

	return best_result
}

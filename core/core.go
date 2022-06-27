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

const db_config_path string = "./core/db-config.json"

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
	// Reading db configuration information file
	db_config_file, err := os.Open(db_config_path)
	if err != nil {
		return nil, err
	}

	db_config_byte, err := ioutil.ReadAll(db_config_file)
	if err != nil {
		return nil, err
	}

	// Convert json format to golang map format
	var db_config map[string]string
	json.Unmarshal(db_config_byte, &db_config)

	// db settings
	cfg := mysql.Config{
		User:                 db_config["User"],
		Passwd:               db_config["Passwd"],
		Net:                  db_config["Net"],
		Addr:                 db_config["Addr"],
		DBName:               db_config["DBName"],
		AllowNativePasswords: true,
	}

	// open mysql db
	db, dberror := sql.Open("mysql", cfg.FormatDSN())
	if dberror != nil {
		return nil, err
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
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Open PW file error!!",
		})
		return
	}
	password_byte, err := ioutil.ReadAll(password_file)
	if err != nil {
		//Handle Error
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": "Read PW file error!!",
		})
		return
	}
	// Convert json format to golang format
	var pw Passwd
	json.Unmarshal(password_byte, &pw)

	// compare password with sotred password
	bcryprterr := bcrypt.CompareHashAndPassword([]byte(pw.Key), []byte(newData.Password))

	if bcryprterr == nil {
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

	// The part that calls the position estimation operation
	ch := make(chan []*result.ResultData)
	slice_len := int(math.Ceil(float64(len(db_pos_arr)) / 3))

	for i := 0; i < 2; i++ {
		go CalcPos(db_pos_arr[slice_len*i:slice_len*(i+1)], newData, 0.6, ch)
	}
	go CalcPos(db_pos_arr[slice_len*2:], newData, 0.6, ch)

	var result_arr []*result.ResultData
	for i := 0; i < 3; i++ {
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

	largest_count := result_list[0].Count
	var top_result_arr []*result.ResultData
	for i, result_len, result_data := 0, len(result_list), result_list.Pop().(*result.ResultData); i < result_len && float64(result_data.Count) > (float64(largest_count)*margin); i++ {
		top_result_arr = append(top_result_arr, result_data)
		result_data = result_list.Pop().(*result.ResultData)
	}

	sort.Slice(top_result_arr, func(i, j int) bool {
		return top_result_arr[i].Ratio < top_result_arr[j].Ratio
	})

	ch <- top_result_arr
}

func CalcKnn(result_arr []*result.ResultData, k int) *result.ResultData {
	k_count := make(map[string]int)
	for i := 0; i < k && i < len(result_arr); i++ {
		if _, ok := k_count[result_arr[i].Position]; ok {
			k_count[result_arr[i].Position]++
		} else {
			k_count[result_arr[i].Position] = 1
		}
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

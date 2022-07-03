package main

import (
	"runtime"

	"github.com/chromato99/WiFi_Positioning_Server_go/core"

	"github.com/gin-gonic/gin"
)

func main() {
	runtime.GOMAXPROCS(4)
	router := gin.Default()
	router.POST("/test", core.Test)
	router.POST("/add", core.AddData)
	router.POST("/findPosition", core.FindPosition)

	router.Run(":8004")
}

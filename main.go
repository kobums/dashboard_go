package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/global/setting"
	"dashboard/models"
	"dashboard/services"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.New(rand.NewSource(time.Now().UnixNano()))

	log.Info().Str("Version", config.Version).Str("Mode", config.Mode).Msg("Start")

	models.InitCache()

	tempPath := fmt.Sprintf("%v/temp", config.UploadPath)
	os.MkdirAll(tempPath, 777)
	os.Chmod(tempPath, os.FileMode(0755))

	setting.GetInstance()

	services.Http()
}
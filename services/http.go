package services

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"dashboard/global/config"
	"dashboard/global/log"
	"dashboard/router"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func Http() {
	log.Info().Str("service", "HTTP").Msg("Start Service")

	app := fiber.New(fiber.Config{
		BodyLimit:             500 * 1024 * 1024,
		Prefork:               false,
		CaseSensitive:         true,
		StrictRouting:         true,
		DisableStartupMessage: true,
		JSONEncoder:           json.Marshal,
		JSONDecoder:           json.Unmarshal,
	})

	sites := strings.Join(config.Cors, ", ")
	if sites != "" {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     sites,
			AllowCredentials: true,
			AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
			AllowHeaders:     "Origin,Content-Type,Accept,Content-Length,Accept-Language,Accept-Encoding,Connection,Access-Control-Allow-Origin",
		}))
	}

	if config.Log.Web {
		app.Use(fiberzerolog.New(fiberzerolog.Config{
			Logger: log.Get(),
		}))
	}

	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestCompression,
	}))


	// app.Static("/webdata", config.DocumentRoot)
	app.Static("/webdata", config.UploadPath)

	// 업로드 이미지 정적 서빙: /uploads/{파일명} → {UploadPath}/uploads/{파일명}
	app.Static("/uploads", config.UploadPath+"/uploads")

	// SPA(dist) 정적 서빙 — /api 외 나머지 경로는 아래 catch-all 이 index.html 로 폴백
	app.Static("/", config.DocumentRoot)

	router.SetRouter(app)

	app.Get("/*", func(ctx *fiber.Ctx) error {
		return ctx.SendFile(fmt.Sprintf("./%v/index.html", config.DocumentRoot), true)
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-c
		log.Info().Msg("Gracefully shutting down...")
		_ = app.Shutdown()
	}()

	if config.Mode == "develop" || !config.Tls.Use {
		if err := app.Listen(":" + config.Port); err != nil {
			log.Error().Msg(err.Error())
		}
	} else {
		cer, err := tls.LoadX509KeyPair(config.Tls.Cert, config.Tls.Key)
		if err != nil {
			log.Error().Msg("TLS error")
			log.Error().Msg(err.Error())
			return
		}

		cert := &tls.Config{Certificates: []tls.Certificate{cer}}

		ln, err := tls.Listen("tcp", ":"+config.Port, cert)
		if err != nil {
			log.Error().Msg(err.Error())
			return
		}

		if err := app.Listener(ln); err != nil {
			log.Error().Msg(err.Error())
		}
	}
}

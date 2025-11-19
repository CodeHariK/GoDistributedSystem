package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func greet(c fiber.Ctx) error {
	return c.SendString("Hello, World")
}

func fiberServer() {
	go func() {
		port := 8080
		fmt.Printf("Server running on http://localhost:%d\n", port)

		app := fiber.New(fiber.Config{
			CaseSensitive:             true,
			StrictRouting:             true,
			DisableDefaultDate:        true,
			DisableHeaderNormalizing:  true,
			DisableDefaultContentType: true,
		})

		app.Get("/", greet)

		log.Fatal(app.Listen(":" + strconv.Itoa(port)))
	}()
}

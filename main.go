package main

import (
	"log"
	"text/template"

	"github.com/TylerConlee/TicketPulse/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB
var err error
var templates *template.Template

func init() {
	db, err = gorm.Open(sqlite.Open("ticketpulse.db"), &gorm.Config{})
	if err != nil {
		log.Panic(err)
	}
	db.AutoMigrate(&models.User{})
}

func main() {
	StartServer()
}

package main

import (
	"log"
	"ticketpulse/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB
var err error

func init() {
	db, err = gorm.Open(sqlite.Open("ticketpulse.db"), &gorm.Config{})
	if err != nil {
		log.Panic(err)
	}
	db.AutoMigrate(&models.User{})
}

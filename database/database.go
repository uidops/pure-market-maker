package database

import "gorm.io/gorm"

var DB *gorm.DB

type Order struct {
	gorm.Model
	Exchange   string
	Market     string
	Side       string
	Volume     float64
	Price      float64
	MsamexID   int64  `gorm:"default:0"`
	MsamexUUID string `gorm:"default:"`
}

type Best struct {
	gorm.Model
	Exchange string
	Market   string
	Side     string
	Volume   float64
	Price    float64
	Rank     int `gorm:"-"`
}

type Last struct {
	gorm.Model
	Exchange string
	Market   string
	Side     string
	Volume   float64
	Price    float64
	Rank     int `gorm:"-"`
}

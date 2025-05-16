package models

import "time"

type Order struct {
	ID         int       `json:"id"`
	Price      int       `json:"price"`
	Created_at time.Time `json:"created_at"`
}

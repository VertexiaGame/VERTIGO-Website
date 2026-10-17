package models

import "time"

const (
	BoughtViaTrade    = "trade"
	BoughtViaFree     = "free"
	BoughtViaVertices = "vertices"
	BoughtViaTickets  = "tickets"
	BoughtViaAwarded  = "awarded"
)

var ValidBoughtVia = map[string]bool{
	BoughtViaTrade:    true,
	BoughtViaFree:     true,
	BoughtViaVertices: true,
	BoughtViaTickets:  true,
	BoughtViaAwarded:  true,
}

type Crate struct {
	ID    int       `json:"id"`
	IID   int       `json:"iid"`
	UID   int       `json:"uid"`
	BV    string    `json:"bv"`
	AID   int       `json:"aid"`
	Price int       `json:"price"`
	Date  time.Time `json:"date"`
}

type PurchaseRequest struct {
	Currency string `json:"currency"`
}

type PurchaseResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Crate   *Crate `json:"crate,omitempty"`
	Bucks   int    `json:"bucks,omitempty"`
	Bits    int    `json:"bits,omitempty"`
}

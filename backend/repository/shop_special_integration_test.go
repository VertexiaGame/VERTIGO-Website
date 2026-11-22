package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"vertexia-frontend/backend/models"
)

func TestSpecialItemLifecycleMySQL(t *testing.T) {
	if os.Getenv("VERTIGO_TEST_MYSQL") != "1" {
		t.Skip("set VERTIGO_TEST_MYSQL=1 to run the MySQL integration test")
	}
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal(err)
	}
	cfg := mysql.NewConfig()
	cfg.User, cfg.Passwd, cfg.Net = os.Getenv("DB_USER"), os.Getenv("DB_PASS"), "tcp"
	cfg.Addr = os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT")
	cfg.ParseTime = true
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	cfg.DBName = fmt.Sprintf("vertigo_special_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + cfg.DBName); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE " + cfg.DBName); err != nil {
			t.Error(err)
		}
	}()
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	applySQLSections(t, db, "shop_special fixtures", "shop")
	for _, query := range []string{
		`INSERT INTO users VALUES (1, 'creator', 0, 0, 0), (2, 'private', 1, 100, 0),
			(3, 'buyer', 0, 100, 0), (4, 'poor', 0, 0, 0), (5, 'other', 0, 100, 0), (6, 'public', 0, 100, 0)`,
		`INSERT INTO shopitem (id, special, stock, original_stock) VALUES (1, 'true', 2, 2)`,
		`INSERT INTO crate (iid, uid, bv) VALUES (1, 6, 'awarded')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewShopRepository(db)
	legacy, err := repo.GetSpecialCopy(1, 6)
	if err != nil || legacy == nil || legacy.Serial != 1 {
		t.Fatalf("legacy copy = %+v, %v", legacy, err)
	}
	var remaining int
	if err := db.QueryRow(`SELECT stock FROM shopitem WHERE id = 1`).Scan(&remaining); err != nil || remaining != 1 {
		t.Fatalf("legacy stock = %d, %v", remaining, err)
	}

	itemID, err := repo.Create(1, "Special shirt", "", "shirts", 10, 0, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateApproval(itemID, models.ShopApprovalApproved); err != nil {
		t.Fatal(err)
	}
	first, err := repo.PurchaseItem(2, itemID, "vertices")
	if err != nil || first == nil || first.Serial != 1 {
		t.Fatalf("first purchase = %+v, %v", first, err)
	}
	if err := repo.SetSpecialListing(2, itemID, 1, 50, 0); !errors.Is(err, models.ErrSpecialNotResellable) {
		t.Fatalf("listing before sellout: %v", err)
	}
	if _, err := repo.PurchaseItem(2, itemID, "vertices"); err == nil || err.Error() != "already owned" {
		t.Fatalf("duplicate primary purchase: %v", err)
	}
	type purchase struct {
		userID int
		crate  *models.Crate
		err    error
	}
	primary := make(chan purchase, 2)
	for _, userID := range []int{5, 6} {
		go func() {
			crate, err := repo.PurchaseItem(userID, itemID, "vertices")
			primary <- purchase{userID, crate, err}
		}()
	}
	winner, loser := <-primary, <-primary
	if winner.err != nil {
		winner, loser = loser, winner
	}
	if winner.err != nil || winner.crate == nil || winner.crate.Serial != 2 || loser.err == nil || loser.err.Error() != "item is out of stock" {
		t.Fatalf("last primary stock: %+v, %+v", winner, loser)
	}
	if err := repo.SetSpecialListing(3, itemID, 1, 50, 0); !errors.Is(err, models.ErrSpecialNotOwner) {
		t.Fatalf("listing another user's serial: %v", err)
	}
	for _, update := range []struct {
		price   int
		version int64
	}{{50, 0}, {0, 1}, {50, 2}} {
		if err := repo.SetSpecialListing(2, itemID, 1, update.price, update.version); err != nil {
			t.Fatal(err)
		}
	}
	owners, err := repo.GetSpecialOwners(itemID, 10, 0)
	if err != nil || len(owners) != 1 || owners[0].UserID != winner.userID || owners[0].Serial != 2 {
		t.Fatalf("public owners = %+v, %v", owners, err)
	}
	listings, err := repo.GetSpecialListings(itemID, 10, 0)
	if err != nil || len(listings) != 1 || listings[0].SellerID != 0 || listings[0].SellerName != "" || listings[0].Version != 3 {
		t.Fatalf("listings = %+v, %v", listings, err)
	}
	for _, attempt := range []struct {
		buyer   int
		version int64
		want    error
	}{
		{3, 1, models.ErrSpecialListingChanged},
		{2, 3, models.ErrSpecialOwnListing},
		{winner.userID, 3, models.ErrSpecialAlreadyOwned},
		{4, 3, models.ErrSpecialInsufficientFunds},
	} {
		if _, err := repo.BuySpecialListing(attempt.buyer, itemID, 1, attempt.version); !errors.Is(err, attempt.want) {
			t.Fatalf("buyer %d: got %v, want %v", attempt.buyer, err, attempt.want)
		}
	}
	if _, err := db.Exec(`INSERT INTO avatar (id, shirt) VALUES (2, ?)`, itemID); err != nil {
		t.Fatal(err)
	}
	resales := make(chan purchase, 2)
	for _, userID := range []int{3, loser.userID} {
		go func() {
			seller, err := repo.BuySpecialListing(userID, itemID, 1, 3)
			if err == nil && seller != 2 {
				err = fmt.Errorf("unexpected seller %d", seller)
			}
			resales <- purchase{userID: userID, err: err}
		}()
	}
	buyer, failed := <-resales, <-resales
	if buyer.err != nil {
		buyer, failed = failed, buyer
	}
	if buyer.err != nil || !errors.Is(failed.err, models.ErrSpecialListingChanged) {
		t.Fatalf("concurrent resale: %+v, %+v", buyer, failed)
	}
	if repo.HasItem(2, itemID) {
		t.Fatal("seller still owns the sold serial")
	}
	received, err := repo.GetSpecialCopy(itemID, buyer.userID)
	if err != nil || received == nil || received.Serial != 1 || received.Price != 0 {
		t.Fatalf("transferred copy = %+v, %v", received, err)
	}
	for userID, want := range map[int]int{1: 20, 2: 140, 4: 0, buyer.userID: 50, failed.userID: 100} {
		var balance int
		if err := db.QueryRow(`SELECT bucks FROM users WHERE id = ?`, userID).Scan(&balance); err != nil || balance != want {
			t.Fatalf("user %d balance = %d, %v, want %d", userID, balance, err, want)
		}
	}
	var equipped int
	if err := db.QueryRow(`SELECT shirt FROM avatar WHERE id = 2`).Scan(&equipped); err != nil || equipped != 0 {
		t.Fatalf("seller's equipped shirt = %d, %v", equipped, err)
	}
}

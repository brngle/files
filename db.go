package files

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/alioygur/gores"
	"github.com/sqids/sqids-go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func OpenDatabase(path string) error {
	it, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}
	db = it

	err = db.AutoMigrate(
		&ShareCode{},
	)
	if err != nil {
		return err

	}

	return nil
}

func ErrorResponse(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		gores.Error(w, http.StatusNotFound, "not found")
		return
	}

	gores.Error(w, http.StatusInternalServerError, "something went wrong")
}

type ShareCode struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	Volume    string    `json:"volume"`
	Path      string    `json:"path"`
	ExpiresAt time.Time `json:"expires_at"`
}

var sqid *sqids.Sqids

func init() {
	it, err := sqids.New()
	if err != nil {
		panic(err)
	}
	sqid = it
}

func (s *ShareCode) Code() string {
	id, _ := sqid.Encode([]uint64{uint64(s.Id)})
	return id
}

func (s *ShareCode) URL(root string) string {
	return fmt.Sprintf("%ss/%s?raw", root, s.Code())
}

func MakeShareCode(volume, path string) (*ShareCode, error) {
	var existing ShareCode
	err := db.First(&existing, "volume = ? AND path = ?", volume, path).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		return &existing, nil
	}

	shareCode := &ShareCode{
		Volume:    volume,
		Path:      path,
		ExpiresAt: time.Now().Add(time.Hour * 24 * 365 * 100),
	}

	err = db.Create(shareCode).Error
	if err != nil {
		return nil, err
	}

	return shareCode, nil
}

func GetShareCode(code string) (*ShareCode, error) {
	id := sqid.Decode(code)
	if len(id) != 1 {
		return nil, errors.New("invalid share code")
	}

	var shareCode ShareCode
	err := db.Find(&shareCode, "id = ?", id[0]).Error
	if err != nil {
		return nil, err
	}

	return &shareCode, nil
}

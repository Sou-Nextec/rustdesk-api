package service

import (
	"io"
	"testing"

	"github.com/lejianwen/rustdesk-api/v2/model"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNextecCleanId(t *testing.T) {
	cases := map[string]string{
		"268 304 385":           "268304385",
		" 268304385 ":           "268304385",
		"268\u00a0304\u00a0385": "268304385",
		"abc def":               "abcdef",
		"":                      "",
	}
	for in, want := range cases {
		if got := NextecCleanId(in); got != want {
			t.Fatalf("NextecCleanId(%q) = %q, quero %q", in, got, want)
		}
	}
}

func TestNextecNormalizeIds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ids?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Skip("sqlite indisponível: ", err)
	}
	DB = db
	l := log.New()
	l.SetOutput(io.Discard)
	Logger = l
	if err := db.AutoMigrate(&model.Peer{}, &model.AddressBook{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.Peer{Id: "536 822 159", Uuid: "a"})
	db.Create(&model.Peer{Id: "111 222 333", Uuid: "b"})
	db.Create(&model.Peer{Id: "111222333", Uuid: "c"}) // conflito: não pode ser sobrescrito
	db.Create(&model.Peer{Id: "999000111", Uuid: "d"})
	db.Create(&model.AddressBook{Id: "700 800 900", UserId: 1})

	NextecNormalizeIds()

	var n int64
	db.Model(&model.Peer{}).Where("id = ?", "536822159").Count(&n)
	if n != 1 {
		t.Fatalf("ID com espaço não foi corrigido")
	}
	db.Model(&model.Peer{}).Where("id = ?", "111 222 333").Count(&n)
	if n != 1 {
		t.Fatalf("ID em conflito deveria ficar como estava")
	}
	db.Model(&model.Peer{}).Where("id = ?", "111222333").Count(&n)
	if n != 1 {
		t.Fatalf("o ID já correto foi alterado")
	}
	db.Model(&model.AddressBook{}).Where("id = ?", "700800900").Count(&n)
	if n != 1 {
		t.Fatalf("acesso salvo com espaço não foi corrigido")
	}
}

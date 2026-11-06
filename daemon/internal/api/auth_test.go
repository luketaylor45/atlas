package api

import (
	"strconv"
	"testing"
	"time"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

func TestTicket(t *testing.T) {
	config.NodeConfig.NodeToken = "0123456789abcdef0123"
	exp := time.Now().Add(time.Minute).Unix()
	good := strconv.FormatInt(exp, 10) + "." + signTicket("srv-1", exp)
	if !ValidTicket(good, "srv-1") {
		t.Fatal("valid ticket rejected")
	}
	if ValidTicket(good, "srv-2") {
		t.Fatal("ticket accepted for another server")
	}
	old := time.Now().Add(-time.Minute).Unix()
	if ValidTicket(strconv.FormatInt(old, 10)+"."+signTicket("srv-1", old), "srv-1") {
		t.Fatal("expired ticket accepted")
	}
	config.NodeConfig.NodeToken = "change-me"
	if ValidTicket(good, "srv-1") {
		t.Fatal("placeholder token accepted")
	}
}

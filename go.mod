module github.com/LightningTipBot/LightningTipBot

go 1.17

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/btcsuite/btcd/btcec/v2 v2.2.0
	github.com/eko/gocache v1.2.0
	github.com/fiatjaf/go-lnurl v1.11.2
	github.com/fiatjaf/ln-decodepay v1.1.0
	github.com/gorilla/mux v1.8.0
	github.com/imroc/req v0.3.0
	github.com/jinzhu/configor v1.2.1
	github.com/makiuchi-d/gozxing v0.0.2
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/nicksnyder/go-i18n/v2 v2.1.2
	github.com/orcaman/concurrent-map v1.0.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/tidwall/buntdb v1.2.7
	github.com/tidwall/gjson v1.12.1
	github.com/tidwall/sjson v1.2.4
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f
	golang.org/x/text v0.3.7
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	gopkg.in/lightningtipbot/telebot.v3 v3.0.0-20220326213923-f323bb71ac8e
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.12
)

require (
	github.com/XiaoMi/pegasus-go-client v0.0.0-20210427083443-f3b6b08bc4c2 // indirect
	github.com/aead/siphash v1.0.1 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b // indirect
	github.com/bshuster-repo/logrus-logstash-hook v1.0.2 // indirect
	github.com/btcsuite/btcd v0.23.1 // indirect
	github.com/btcsuite/btcd/btcutil v1.1.1 // indirect
	github.com/btcsuite/btcd/btcutil/psbt v1.1.4 // indirect
	github.com/btcsuite/btcd/chaincfg/chainhash v1.0.1 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/btcsuite/btcwallet v0.15.1 // indirect
	github.com/btcsuite/btcwallet/wallet/txauthor v1.2.3 // indirect
	github.com/btcsuite/btcwallet/wallet/txrules v1.2.0 // indirect
	github.com/btcsuite/btcwallet/wallet/txsizes v1.1.0 // indirect
	github.com/btcsuite/btcwallet/walletdb v1.4.0 // indirect
	github.com/btcsuite/btcwallet/wtxmgr v1.5.0 // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/cenkalti/backoff/v4 v4.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/decred/dcrd/lru v1.0.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-errors/errors v1.0.1 // indirect
	github.com/go-redis/redis/v8 v8.8.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.2 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/lightninglabs/gozmq v0.0.0-20191113021534-d20a764486bf // indirect
	github.com/lightninglabs/neutrino v0.14.2 // indirect
	github.com/lightningnetwork/lnd v0.15.0-beta // indirect
	github.com/lightningnetwork/lnd/clock v1.1.0 // indirect
	github.com/lightningnetwork/lnd/queue v1.1.0 // indirect
	github.com/lightningnetwork/lnd/ticker v1.1.0 // indirect
	github.com/lightningnetwork/lnd/tlv v1.0.3 // indirect
	github.com/lightningnetwork/lnd/tor v1.0.1 // indirect
	github.com/magefile/mage v1.9.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.5 // indirect
	github.com/miekg/dns v1.1.43 // indirect
	github.com/nbd-wtf/ln-decodepay v1.5.1 // indirect
	github.com/pegasus-kv/thrift v0.13.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/tidwall/btree v0.6.1 // indirect
	github.com/tidwall/grect v0.1.3 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/rtred v0.1.2 // indirect
	github.com/tidwall/tinyqueue v0.1.1 // indirect
	go.elastic.co/ecslogrus v1.0.0 // indirect
	go.opentelemetry.io/otel v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v0.20.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/sys v0.0.0-20220817070843-5a390386f1f2 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apimachinery v0.0.0-20191123233150-4c4803ed55e3 // indirect
)

// replace gopkg.in/lightningtipbot/telebot.v2 => ../telebot

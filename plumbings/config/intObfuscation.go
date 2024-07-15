package config

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pjebs/optimus-go"
)

const defaultObfuscationPrime uint64 = 3714939857
const defaultObfuscationRandom uint64 = 4237402219

// intObfuscation wraps the underlying optimus.Optimus,
// providing integer obfuscation being wrapped with
// hex encoding, producing 8 digit obfuscated hex
type intObfuscation struct {
	o      *optimus.Optimus
	mu     sync.Mutex
	Prime  uint64 `yaml:"Prime" env:"OBFUSCATION_PRIME"`
	Random uint64 `yaml:"Random" env:"OBFUSCATION_RANDOM"`
}

// Do not encode negative integers
//
// The accaptable range should be (MaxUint32)/2
//
// This wrapper exist more as a convenience function
// such as no need to typecast common int to uint64
func (o *intObfuscation) Encode(input int) string {
	if o.o == nil {
		o.init()
	}
	ui64 := uint64(input)
	ui64obs := o.o.Encode(ui64)
	return fmt.Sprintf("%08x", ui64obs)
}
func (o *intObfuscation) Decode(input string) (out int, err error) {
	if o.o == nil {
		o.init()
	}
	i64obs, err := strconv.ParseInt(input, 16, 64)
	if err != nil {
		return
	}
	ui64 := o.o.Decode(uint64(i64obs))
	out = int(ui64)
	return
}
func (o *intObfuscation) init() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.o != nil {
		return
	}
	var secret optimus.Optimus
	if o.Prime > 0 && o.Random > 0 {
		secret = optimus.NewCalculated(o.Prime, o.Random)
	} else {
		secret = optimus.NewCalculated(defaultObfuscationPrime, defaultObfuscationRandom)
	}
	o.o = &secret
}

type ObfuscatedInt int

func (o ObfuscatedInt) MarshalText() ([]byte, error) {
	return []byte(Config.IntObfuscation.Encode(int(o))), nil
}

func (u *ObfuscatedInt) UnmarshalText(txt []byte) (err error) {
	i, err := Config.IntObfuscation.Decode(string(txt))
	if err != nil {
		return
	}
	*u = ObfuscatedInt(i)
	return nil
}
func (u *ObfuscatedInt) Parse(str string) (err error) {
	i, err := Config.IntObfuscation.Decode(str)
	if err != nil {
		return
	}
	*u = ObfuscatedInt(i)
	return nil
}
func (u *ObfuscatedInt) IsEmpty() bool {
	return *u == ObfuscatedInt(0)
}

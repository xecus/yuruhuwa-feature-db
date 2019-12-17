package data

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	//"errors"
	//"fmt"
	"math"
	"strings"
)

func generateRandomVal(min float64, max float64) float64 {
	//rand.Seed(time.Now().UnixNano())
	return rand.Float64()*(max-min) + min
}

type PosVector struct {
	Vals []float64
	Hash string
}

func (ps *PosVector) InitVector(useRandom bool, size int) error {
	ps.Vals = make([]float64, size, size)
	if useRandom {
		for i, _ := range ps.Vals {
			ps.Vals[i] = generateRandomVal(-1.0, 1.0)
		}
	} else {
		for i, _ := range ps.Vals {
			ps.Vals[i] = 0.0
		}
	}
	return nil
}

func (ps *PosVector) LoadPosition(pv *PosVector) error {
	for i, _ := range ps.Vals {
		ps.Vals[i] = pv.Vals[i]
	}
	ps.CalcHash()
	return nil
}

func (ps *PosVector) LoadPositionFromArray(pv [512]float64) error {
	for i, _ := range ps.Vals {
		ps.Vals[i] = pv[i]
	}
	ps.CalcHash()
	return nil
}

func (ps *PosVector) CalcHash() (string, error) {
	if ps.Hash == "" {
		length := 512 * 8
		binaries := make([]byte, length, length)
		for i, _ := range ps.Vals {
			tmp := make([]byte, 8)
			bits := math.Float64bits(ps.Vals[i])
			binary.LittleEndian.PutUint64(tmp, bits)
			for j, _ := range tmp {
				binaries[i*8+j] = tmp[j]
			}
		}
		result := sha256.Sum256(binaries)
		ps.Hash = strings.ToUpper(hex.EncodeToString(result[:]))
	}
	return ps.Hash, nil
}

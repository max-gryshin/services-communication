package utils

import (
	"math/rand"
	"strconv"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var n string
var pMin int

func SetUpUtils(appName string, portMin int) {
	n = appName
	pMin = portMin
}

func RandStringBytesMask(n int) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func GetRandStrings() []string {
	res := make([]string, 2)
	for i := 0; i < 2; i++ {
		res = append(res, RandStringBytesMask(10))
	}

	return res
}

func GetServiceName(port string) string {
	portInt, _ := strconv.Atoi(port)

	return n + strconv.Itoa(portInt-pMin) + ":" + port
}

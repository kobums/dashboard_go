package global

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"dashboard/global/config"
	"dashboard/global/log"

	"github.com/google/uuid"
	"golang.org/x/exp/constraints"
)

func ToMap(slice []string) map[string]int {
	m := map[string]int{}
	for i, x := range slice {
		m[x] = i
	}
	return m
}

func ReverseMap(inmap map[int]string) map[string]int {
	outmap := make(map[string]int)
	for k, v := range inmap {
		outmap[v] = k
	}
	return outmap
}

func Atoi(value string) int {
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, " ", "")
	i, _ := strconv.Atoi(value)
	return i
}

func Atol(value string) int64 {
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, " ", "")
	i, _ := strconv.ParseInt(value, 10, 64)
	return i
}

func Atof(value string) float64 {
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ReplaceAll(value, " ", "")
	i, _ := strconv.ParseFloat(strings.Replace(value, ",", "", -1), 64)
	return i
}

func Itoa(value int) string {
	return fmt.Sprintf("%v", value)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func ArrayToString(A []int, delim string) string {
	var buffer bytes.Buffer
	for i := 0; i < len(A); i++ {
		buffer.WriteString(strconv.Itoa(A[i]))
		if i != len(A)-1 {
			buffer.WriteString(delim)
		}
	}

	return buffer.String()
}

func StringToArray(value string, delim string) []int {
	ret := make([]int, 0)

	data := strings.Split(value, delim)

	for _, v := range data {
		ret = append(ret, Atoi(v))
	}

	return ret
}

func UUID() string {
	u2 := uuid.New()
	return u2.String()
}

func GetTempFilename() string {
	return filepath.Join("webdata/temp", UUID())
}

func Duration(seconds int) string {
	h := seconds / 60 / 60
	m := seconds / 60 % 60
	s := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func StripTags(content string) string {
	re := regexp.MustCompile(`<(.|\n)*?>`)
	return re.ReplaceAllString(content, "")
}

func FindImages(htm string) []string {
	var imgRE = regexp.MustCompile(`<img[^>]+\bsrc=["']([^"']+)["']`)
	imgs := imgRE.FindAllStringSubmatch(htm, -1)
	out := make([]string, len(imgs))
	for i := range out {
		out[i] = imgs[i][1]
	}
	return out
}

func FindImage(htm string) string {
	var imgRE = regexp.MustCompile(`<img[^>]+\bsrc=["']([^"']+)["']`)
	imgs := imgRE.FindAllStringSubmatch(htm, -1)

	if len(imgs) == 0 {
		return ""
	}

	return imgs[0][1]
}

func IsEmptyDate(date string) bool {
	if date == "" || date == "0000-00-00 00:00:00" || date == "1000-01-01 00:00:00" {
		return true
	} else {
		return false
	}
}

func GetSha256(str string) string {
	hash := sha256.New()

	hash.Write([]byte(str))
	md := hash.Sum(nil)
	return hex.EncodeToString(md)
}

func SendSMS(tel string, content string) bool {
	str := fmt.Sprintf("user_id=%v&key=%v&sender=%v&receiver=%v&msg=%v", config.Sms.User, config.Sms.Key, config.Sms.Sender, tel, content)

	rqb := bytes.NewBufferString(str)
	rq, e := http.NewRequest("POST", "https://apis.aligo.in/send/", rqb)
	if e != nil {
		log.Error().Msg(e.Error())
		return false
	}
	rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	hc := &http.Client{Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	rs, e := hc.Do(rq)
	if e != nil {
		return false
	}

	defer rs.Body.Close()

	c, e := io.ReadAll(rs.Body)
	if e != nil {
		log.Error().Msg(e.Error())
		return false
	}

	log.Println(string(c))

	return true
}

func WriteFile(filename string, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}

func ReadFile(filename string) string {
	dat, err := os.ReadFile(filename)

	if err != nil {
		return ""
	}

	return string(dat)
}

func Substr(str string, start int, end int) string {
	b := []byte(str)
	idx := 0
	length := 0
	for i := 0; i < start; i++ {
		_, size := utf8.DecodeRune(b[idx:])

		if size == 3 {
			length += 2
		} else {
			length++
		}

		if length >= start {
			break
		}
		idx += size
	}

	pos1 := idx
	idx = 0
	length = 0
	for i := 0; i < end; i++ {
		_, size := utf8.DecodeRune(b[idx:])

		if size == 3 {
			length += 2
		} else {
			length++
		}

		if length >= end {
			break
		}
		idx += size
	}

	return str[pos1:idx]
}

func Strlen(s string) int {
	length := len(s)
	r := utf8.RuneCountInString(s)

	return r + (length-r)/2
}

func DownloadImage(url string, filename string) int64 {
	file, err := os.Create(filename)

	if err != nil {
		log.Error().Msgf("download image error: %v", url)
		return 0
	}

	defer file.Close()

	resp, err := http.Get(url)

	if err != nil {
		log.Error().Msg(err.Error())
		return 0
	}

	defer resp.Body.Close()

	size, err := io.Copy(file, resp.Body)

	if err != nil {
		log.Error().Msg(err.Error())
		return 0
	}

	if size == 0 {
		os.Remove(filename)
	}

	return size
}

func MakeUniqueSlice[T constraints.Integer | constraints.Float | string](arr []T) []T {
	ret := make([]T, 0)

	m := make(map[T]bool)

	for _, val := range arr {
		if _, ok := m[val]; !ok {
			m[val] = true
			ret = append(ret, val)
		}
	}

	return ret
}

func JsonEncode(item interface{}) string {
	b, err := json.Marshal(item)
	if err != nil {
		log.Error().Msg(err.Error())
	}
	return string(b)
}

func JsonDecode(str string, item interface{}) error {
	err := json.Unmarshal([]byte(str), item)
	if err != nil {
		log.Error().Msg(err.Error())
		return err
	}

	return nil
}

func MakeSearchKeyword(str string) []string {
	re := regexp.MustCompile(`[^ a-zA-Z0-9ㄱ-ㅎㅏ-ㅣ가-힣]+`)
	keyword := re.ReplaceAllString(str, " ")

	min := 2
	max := 20

	items := make([]string, 0)

	words := strings.Split(keyword, " ")
	for _, v := range words {
		if v == "" {
			continue
		}

		r := []rune(v)
		length := len(r)
		if length < min {
			continue
		}

		for i := 0; i < length; i++ {
			limit := length - i
			if limit > max {
				limit = max
			}
			for j := min; j <= limit; j++ {
				items = append(items, string(r[i:i+j]))
			}
		}
	}

	items = MakeUniqueSlice(items)
	return items
}

func Reverse[T any](original []T) (reversed []T) {
	reversed = make([]T, len(original))
	copy(reversed, original)

	for i := len(reversed)/2 - 1; i >= 0; i-- {
		tmp := len(reversed) - 1 - i
		reversed[i], reversed[tmp] = reversed[tmp], reversed[i]
	}

	return
}

func XmlEncode(item interface{}) string {
	b, err := xml.Marshal(item)
	if err != nil {
		log.Error().Msg(err.Error())
	}
	return string(b)
}

func XmlDecode(str string, item interface{}) error {
	err := xml.Unmarshal([]byte(str), item)
	if err != nil {
		log.Error().Msg(err.Error())
		return err
	}

	return nil
}

func SendMail(email string, title string, content string) error {

	server := "[EMAIL_SERVER]"
	port := "587"
	user := "[EMAIL_ADDRESS]"
	passwd := "[EMAIL_PASSWORD]"
	sender := "meet <[EMAIL_ADDRESS]>"

	if port == "" {
		port = "587"
	}

	if sender == "" {
		sender = user
	}

	auth := smtp.PlainAuth("", user, passwd, server)

	to := []string{email}
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subject := fmt.Sprintf("Subject: %v\r\n", title)
	body := fmt.Sprintf("%v\r\n", content)
	msg := []byte(subject + mime + "\n" + body)

	mailServer := fmt.Sprintf("%v:%v", server, port)
	err := smtp.SendMail(mailServer, auth, sender, to, msg)
	if err != nil {
		return err
	}

	log.Info().Msgf("ALARM MAIL : %v", email)

	return nil
}

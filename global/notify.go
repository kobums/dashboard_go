package global

type MessageType int

const (
	_ MessageType = iota
	SessionTimeout
	MyCount
)

var Messages = []string{"", "timeout", "mycount"}

type Notify struct {
	Id      int64
	UUID    string
	Message MessageType
}

var _ch chan Notify

func init() {
	_ch = make(chan Notify, 1000)
}

func SendNotify(id int64, UUID string, message MessageType) {
	item := Notify{
		Id:      id,
		UUID:    UUID,
		Message: message,
	}
	_ch <- item
}

func SendNotifys(ids []int64, message MessageType) {
	for _, v := range ids {
		SendNotify(v, "", message)
	}
}

func GetChannel() chan Notify {
	return _ch
}

func GetMessage(message MessageType) string {
	return Messages[message]
}

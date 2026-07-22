package global

var _chCron chan bool

func init() {
	_chCron = make(chan bool, 10)
}

func RestartCron() {
	_chCron <- true
}

func GetCronChannel() chan bool {
	return _chCron
}

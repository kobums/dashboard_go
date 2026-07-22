package setting

import (
	"sync"

	"dashboard/global"
	"dashboard/global/log"
	"dashboard/models"
)

var lock = &sync.Mutex{}

type Instance struct {
	Settings map[string]string
}

var _instance *Instance

func GetInstance() *Instance {
	if _instance == nil {
		lock.Lock()
		defer lock.Unlock()

		if _instance == nil {
			instance := &Instance{}
			instance.Init()

			_instance = instance
		}
	}

	return _instance
}

func (c *Instance) InitSetting() {
	log.Info().Str("model", "setting").Msg("Cache Init")
	conn := models.NewConnection()
	defer conn.Close()

	// settingManager := models.NewSettingManager(conn)
	// settingManager.SelectLog = false
	// settings := settingManager.Find(nil)

	c.Settings = make(map[string]string)
	// for _, v := range settings {
	// 	key := v.Key
	// 	c.Settings[key] = v.Value
	// }
}

func (c *Instance) Init() {
	c.InitSetting()
}

func (c *Instance) Setting(key string) string {
	if value, exist := c.Settings[key]; exist {
		return value
	}

	return ""
}

func (c *Instance) SettingInt(key string) int {
	if value, exist := c.Settings[key]; exist {
		return global.Atoi(value)
	}

	return 0
}

package models

func InitCache() {
    db := GetConnection()

    if db == nil {
        return
    }

    defer db.Close()

}

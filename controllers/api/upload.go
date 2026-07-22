package api

import (
	"dashboard/controllers"
	"dashboard/global/config"
	"os"
	"path"
)

type UploadController struct {
	controllers.Controller
}

// @POST()
func (c *UploadController) Index() {
	name := c.Context.FormValue("name", "upload")

	dirName := c.Now.DateAsOnlyNumber()
	fullPath := path.Join(config.UploadPath, dirName)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		err = os.Mkdir(fullPath, os.ModePerm)
		if err != nil {
			c.Error(err)
			return
		}
	}

	originalFilename, filename := c.GetUpload(dirName, name)

	c.Set("filename", filename)
	c.Set("original", originalFilename)
}
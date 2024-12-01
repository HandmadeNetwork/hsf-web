package website

import (
	"bytes"
	"hsf/src/ee"
	"hsf/src/templates"
	"net/http"
)

func renderHTML(c *RequestContext, templateName string, templateData any) ResponseData {
	res := ResponseData{
		StatusCode: http.StatusOK,
	}

	err := templates.Render(&res, templateName, templateData)

	if err != nil {
		return render500HTML(c, ee.New(err, "Failed to render template"))
	}

	return res
}

func render500HTML(c *RequestContext, error error) ResponseData {
	res := ResponseData{
		StatusCode: http.StatusInternalServerError,
	}

	c.Logger.Error().Err(error).Msg("Internal server error")

	err := templates.Render(&res, "error500", GetBaseData())
	if err != nil {
		c.Logger.Error().Err(ee.New(err, "Failed to render error500 template")).Msg("Failed to render error page")

		return ResponseData{
			StatusCode: http.StatusInternalServerError,
			Body:       bytes.NewBufferString("Fatal error"),
		}
	}

	return res
}

func render404HTML(c *RequestContext) ResponseData {
	res := ResponseData{
		StatusCode: http.StatusNotFound,
	}

	err := templates.Render(&res, "error404", GetBaseData())
	if err != nil {
		return render500HTML(c, ee.New(err, "Failed to render 404 page"))
	}

	return res
}

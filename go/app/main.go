package main


import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

const (
	ImgDir   = "images"
	JsonFile = "items.json"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	Id 	 int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Image   string `json:"image_name"`
}

type Items struct {
	Items []Item `json:"items"`
}


func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func addItem(c echo.Context) error {
	// Get form data
	name := c.FormValue("name")
	category := c.FormValue("category")
	image ,err:= c.FormFile("image")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Image file is required")
	}

	items, err := getItems()
	if err != nil {
		return err
	}

	hashedimage, err := hashAndSaveImage(image)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

	item := Item{Name: name, Category: category, Image: hashedimage}
	items.Items = append(items.Items, item)
	err = saveItems(items)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("item received: %s, Category: %s ,Image: %s", name, category,hashedimage)
	res := Response{Message: message}
	return c.JSON(http.StatusOK, res)
}

func hashAndSaveImage(fileHeader *multipart.FileHeader) (string, error) {

    src, err := fileHeader.Open()
    if err != nil {
        return "", err
    }
    defer src.Close()

    hasher := sha256.New()
    if _, err := io.Copy(hasher, src); err != nil {
        return "", err
    }
    hashedFilename := hex.EncodeToString(hasher.Sum(nil)) + ".jpg"
    imagePath := filepath.Join(ImgDir, hashedFilename)

    if _, err := os.Stat(ImgDir); os.IsNotExist(err) {
        os.Mkdir(ImgDir, 0755)
    }

    src.Seek(0, io.SeekStart)

    out, err := os.Create(imagePath)
    if err != nil {
        return "", err
    }
    defer out.Close()

    if _, err = io.Copy(out, src); err != nil {
        return "", err
    }

    return hashedFilename, nil
}


func getItems() (Items, error) { 
	var items Items
	file, err := os.ReadFile(JsonFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Items{Items: []Item{}}, nil // ファイルがない場合は空のItemsを返す
		}
		return Items{}, err
	}
	err = json.Unmarshal(file, &items)
	if err != nil {
		return Items{}, err
	}
	return items, nil
}

func getItemsHandler(c echo.Context) error {
	items, err := getItems()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, items)
}

func getitemfromID(c echo.Context) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format")
	}

	items, err := getItems()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	for _, item := range items.Items {
		if item.Id == id {
			return c.JSON(http.StatusOK, item)
		}
	}

	return c.JSON(http.StatusNotFound, Response{Message: "Item not found"})
}


func getImg(c echo.Context) error {
	imgPath := path.Join(ImgDir, c.Param("imageFilename"))

	if !strings.HasSuffix(imgPath, ".jpg") {
		res := Response{Message: "Image path does not end with .jpg"}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, err := os.Stat(imgPath); err != nil {
		c.Logger().Debugf("Image not found: %s", imgPath)
		imgPath = path.Join(ImgDir, "default.jpg")
	}
	return c.File(imgPath)
}


func saveItems(items Items) error {
	file, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(JsonFile, file, 0644)
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)

	frontURL := os.Getenv("FRONT_URL")
	if frontURL == "" {
		frontURL = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{frontURL},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Routes
	e.GET("/", root)
	e.POST("/items", addItem)
	e.GET("/items", getItemsHandler)
	e.GET("/image/:imageFilename", getImg)
	e.GET("items/:id",getitemfromID)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}



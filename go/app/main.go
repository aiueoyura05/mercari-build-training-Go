package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	// "encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ImgDir   = "images"
	JsonFile = "items.json"
	DBfile   = "/Users/yurainagaki/Desktop/mercari-build-training-Go/db/mercari.sqlite3"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Image    string `json:"image_name"`
}

type Items struct {
	Items []Item `json:"items"`
}

func createTable(db *sql.DB) error {

	createTableSQL := `CREATE TABLE IF NOT EXISTS items (
		Id INTEGER PRIMARY KEY AUTOINCREMENT,
		Name TEXT NOT NULL,
		CategoryId INTEGER NOT NULL,
		Image TEXT NOT NULL,
		FOREIGN KEY (CategoryId) REFERENCES categories(Id)
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	categoriesTableSQL := `CREATE TABLE IF NOT EXISTS categories (
        Id INTEGER PRIMARY KEY AUTOINCREMENT,
        Name TEXT NOT NULL
    );`

	if _, err := db.Exec(categoriesTableSQL); err != nil {
		return err
	}
	return nil
}

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func addItem(c echo.Context, db *sql.DB) error {
    name := c.FormValue("name")
    categoryIdStr := c.FormValue("category_id")  // category_id をフォームから取得
    fileHeader, err := c.FormFile("image")
    if err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Image file is required")
    }

    categoryId, err := strconv.Atoi(categoryIdStr)  // 文字列から整数への変換
    if err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid category ID")
    }

    // category_id の存在チェック
    var exists bool
    err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE Id = ?)", categoryId).Scan(&exists)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    if !exists {
        return echo.NewHTTPError(http.StatusBadRequest, "Category not found")
    }

    hashedImageName, err := hashAndSaveImage(fileHeader)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    _, err = db.Exec("INSERT INTO items (Name, CategoryId, Image) VALUES (?, ?, ?)", name, categoryId, hashedImageName)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusOK, echo.Map{
        "message":  "Item added successfully",
        "name":     name,
        "category_id": categoryId,
        "image":    hashedImageName,
    })
}


func addCategory(c echo.Context, db *sql.DB) error {
    // フォームからカテゴリ名を取得
    categoryName := c.FormValue("name")
    if categoryName == "" {
        return echo.NewHTTPError(http.StatusBadRequest, "Category name is required")
    }

    // カテゴリをデータベースに挿入
    _, err := db.Exec("INSERT INTO categories (Name) VALUES (?)", categoryName)
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }

    return c.JSON(http.StatusOK, echo.Map{"message": "Category added successfully"})
}


// func addItem(c echo.Context) error {
// 	// Get form data
// 	name := c.FormValue("name")
// 	category := c.FormValue("category")
// 	image ,err:= c.FormFile("image")
// 	if err != nil {
// 		return echo.NewHTTPError(http.StatusBadRequest, "Image file is required")
// 	}

// 	items, err := getItems()
// 	if err != nil {
// 		return err
// 	}

// 	hashedimage, err := hashAndSaveImage(image)
//     if err != nil {
//         return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
//     }

// 	item := Item{Name: name, Category: category, Image: hashedimage}
// 	items.Items = append(items.Items, item)
// 	err = saveItems(items)
// 	if err != nil {
// 		return err
// 	}

// 	message := fmt.Sprintf("item received: %s, Category: %s ,Image: %s", name, category,hashedimage)
// 	res := Response{Message: message}
// 	return c.JSON(http.StatusOK, res)
// }

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

func getItems(db *sql.DB) ([]Item, error) {
    rows, err := db.Query(`
        SELECT i.Id, i.Name, c.Name AS Category, i.Image 
        FROM items i
        JOIN categories c ON i.CategoryId = c.Id
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var items []Item
    for rows.Next() {
        var item Item
        if err := rows.Scan(&item.Id, &item.Name, &item.Category, &item.Image); err != nil {
            return nil, err
        }
        items = append(items, item)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return items, nil
}


func getItemsHandler(c echo.Context, db *sql.DB) error {
	items, err := getItems(db)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, items)
}

// func getItems() (Items, error) {
// 	var items Items
// 	file, err := os.ReadFile(JsonFile)
// 	if err != nil {
// 		if os.IsNotExist(err) {
// 			return Items{Items: []Item{}}, nil // ファイルがない場合は空のItemsを返す
// 		}
// 		return Items{}, err
// 	}
// 	err = json.Unmarshal(file, &items)
// 	if err != nil {
// 		return Items{}, err
// 	}
// 	return items, nil
// }

func getItemByIDHandler(c echo.Context, db *sql.DB) error {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format")
	}

	item, err := getItemByID(db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, Response{Message: "Item not found"})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, item)
}

func getItemByID(db *sql.DB, id int) (Item, error) {
	var item Item
	row := db.QueryRow("SELECT Id, Name, Category, Image FROM items WHERE Id = ?", id)
	err := row.Scan(&item.Id, &item.Name, &item.Category, &item.Image)
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

// func getitemfromID(c echo.Context) error {
// 	idParam := c.Param("id")
// 	id, err := strconv.Atoi(idParam)
// 	if err != nil {
// 		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID format")
// 	}

// 	items, err := getItems()
// 	if err != nil {
// 		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
// 	}

// 	for _, item := range items.Items {
// 		if item.Id == id {
// 			return c.JSON(http.StatusOK, item)
// 		}
// 	}

// 	return c.JSON(http.StatusNotFound, Response{Message: "Item not found"})
// }

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

// func saveItems(items Items) error {
// 	file, err := json.MarshalIndent(items, "", "  ")
// 	if err != nil {
// 		return err
// 	}
// 	return os.WriteFile(JsonFile, file, 0644)
// }

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	frontURL := os.Getenv("FRONT_URL")
	if frontURL == "" {
		frontURL = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{frontURL},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	//Open DB
	db, err := sql.Open("sqlite3", DBfile)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	err = createTable(db)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Table created successfully")

	// Routes
	e.GET("/", root)
	e.POST("/items", func(c echo.Context) error {
		return addItem(c, db) // Pass db to the handler
	})
	e.GET("/items", func(c echo.Context) error {
		return getItemsHandler(c, db) // Pass db to the handler
	})
	e.GET("/image/:imageFilename", getImg)
	e.GET("/items/:id", func(c echo.Context) error {
		return getItemByIDHandler(c, db) // Pass db to the handler
	})
	e.POST("/categories", func(c echo.Context) error {
		return addCategory(c, db) 
	})

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}

// curl -X POST http://localhost:9000/categories \-F 'name=fashion'
//  curl -X POST http://localhost:9000/items \
//     -F 'name=new jacket' \
//     -F 'category_id=1' \   
//     -F 'images/default.jpg'
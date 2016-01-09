package lobster

import "fmt"

// database objects

type Image struct {
	Id             int
	UserId         int
	Region         string
	Name           string
	Identification string
	Status         string
	SourceVm       int

	Info *ImageInfo
}

// interface objects

type ImageStatus int

const (
	ImagePending ImageStatus = iota
	ImageActive
	ImageError
)

type ImageInfo struct {
	Size    int64
	Status  ImageStatus
	Details map[string]string
}

func imageListHelper(rows Rows) []*Image {
	defer rows.Close()
	images := make([]*Image, 0)
	for rows.Next() {
		image := Image{}
		rows.Scan(&image.Id, &image.UserId, &image.Region, &image.Name, &image.Identification, &image.Status, &image.SourceVm)
		images = append(images, &image)
	}
	return images
}

const IMAGE_QUERY = "SELECT id, user_id, region, name, identification, status, source_vm FROM images"

func imageListAll(db *Database) []*Image {
	return imageListHelper(db.Query(IMAGE_QUERY + " ORDER BY user_id, name"))
}

func imageList(db *Database, userId int) []*Image {
	return imageListHelper(db.Query(IMAGE_QUERY+" WHERE user_id = -1 OR user_id = ? ORDER BY name", userId))
}

func imageListRegion(db *Database, userId int, region string) []*Image {
	return imageListHelper(db.Query(IMAGE_QUERY+" WHERE (user_id = -1 OR user_id = ?) AND region = ? ORDER BY name", userId, region))
}

func imageListVmPending(db *Database, vmId int) []*Image {
	return imageListHelper(db.Query(IMAGE_QUERY+" WHERE source_vm = ? AND status = 'pending'", vmId))
}

func imageGet(db *Database, userId int, imageId int) *Image {
	images := imageListHelper(db.Query(IMAGE_QUERY+" WHERE id = ? AND (user_id = -1 OR user_id = ?)", imageId, userId))
	if len(images) == 1 {
		return images[0]
	} else {
		return nil
	}
}
func imageGetForce(db *Database, imageId int) *Image {
	images := imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images WHERE id = ?", imageId))
	if len(images) == 1 {
		return images[0]
	} else {
		return nil
	}
}

func imageFetch(db *Database, userId int, region string, name string, url string, format string) (int, error) {
	// validate credit
	user := UserDetails(db, userId)
	if user == nil {
		return 0, L.Error("invalid_account")
	} else if user.Credit < MINIMUM_CREDIT {
		return 0, L.Error("insufficient_credit")
	}

	// validate region
	vmi, ok := regionInterfaces[region]
	if !ok {
		return 0, L.Error("invalid_region")
	}

	vmiImage, ok := vmi.(VMIImages)
	if !ok {
		return 0, L.Error("operation_unsupported")
	}

	imageIdentification, err := vmiImage.ImageFetch(url, format)
	if err != nil {
		return 0, err
	} else {
		result := db.Exec("INSERT INTO images (user_id, region, name, identification, status) VALUES (?, ?, ?, ?, 'pending')", userId, region, name, imageIdentification)
		return result.LastInsertId(), nil
	}
}

func imageAdd(db *Database, name string, region string, identification string) {
	db.Exec("INSERT INTO images (name, region, identification) VALUES (?, ?, ?)", name, region, identification)
}

func imageDelete(db *Database, userId int, imageId int) error {
	image := imageGet(db, userId, imageId)
	if image == nil || image.UserId != userId {
		return L.Error("invalid_image")
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return L.Error("operation_unsupported")
	}

	err := vmi.ImageDelete(image.Identification)
	if err != nil {
		return err
	} else {
		db.Exec("DELETE FROM images WHERE id = ?", image.Id)
		return nil
	}
}

func imageDeleteForce(db *Database, imageId int) error {
	image := imageGetForce(db, imageId)
	if image == nil {
		return L.Error("invalid_image")
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return L.Error("operation_unsupported")
	}

	err := vmi.ImageDelete(image.Identification)
	if err != nil {
		ReportError(err, "image force deletion failed", fmt.Sprintf("image_id=%d, identification=%s", image.Id, image.Identification))
	}
	db.Exec("DELETE FROM images WHERE id = ?", image.Id)
	return nil
}

func imageInfo(db *Database, userId int, imageId int) *Image {
	image := imageGet(db, userId, imageId)
	if image == nil || image.UserId != userId {
		return nil
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return nil
	}

	var err error
	image.Info, err = vmi.ImageInfo(image.Identification)
	if err != nil {
		ReportError(err, "imageInfo failed", fmt.Sprintf("image_id=%d, identification=%s", image.Id, image.Identification))
		image.Info = new(ImageInfo)
	}
	return image
}

func imageAutopopulate(db *Database, region string) error {
	if _, ok := regionInterfaces[region]; !ok {
		return fmt.Errorf("specified region %s does not exist", region)
	}
	vmi, ok := regionInterfaces[region].(VMIImages)
	if !ok {
		return L.Error("operation_unsupported")
	}
	images, err := vmi.ImageList()
	if err != nil {
		return err
	}

	// add images that aren't already having matching identification in database
	for _, image := range images {
		var count int
		db.QueryRow(
			"SELECT COUNT(*) FROM images WHERE region = ? AND identification = ?",
			region, image.Identification,
		).Scan(&count)
		if count == 0 {
			imageAdd(db, image.Name, region, image.Identification)
		}
	}

	return nil
}

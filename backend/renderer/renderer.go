package renderer

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/fogleman/fauxgl"
	"github.com/shirou/gopsutil/v3/cpu"
	"vertexia-frontend/backend/models"
)

const (
	Width          = 330
	Height         = 330
	ScaleFactor    = 8
	Brightness     = 0.45
	Roughness      = 1.0
	CamPosX        = 2.0
	CamPosY        = 3.8
	CamPosZ        = 5.3
	CamLookX       = 0.0
	CamLookY       = 1.6
	CamLookZ       = 0.0
	BaseFOV        = 65.0
	DefaultZoom    = 1.05
	AutoZoomMargin = 1.20
	AssetPath      = "./assets/char/"
	FacePath       = "./assets/faces/"
	HeadFile       = "Head.obj"
	TorsoFile      = "Torso.obj"
	LArmFile       = "LeftArm.obj"
	RArmFile       = "RightArm.obj"
	LLegFile       = "LeftLeg.obj"
	RLegFile       = "RightLeg.obj"
	TShirtFile     = "TShirt.obj"
)

const (
	loadThreshold = 95.0
	renderTimeout = 15 * time.Second
)

var (
	preloadmeshes = make(map[string]*fauxgl.Mesh)
	jobq          = make(chan RenderJob, 100)
	getloaddd     float64
	mutexll       sync.RWMutex
	contextPool   = sync.Pool{
		New: func() any {
			return fauxgl.NewContext(Width*ScaleFactor, Height*ScaleFactor)
		},
	}
)

type RenderRequest struct {
	HeadColor      string
	TorsoColor     string
	LeftArmColor   string
	RightArmColor  string
	LeftLegColor   string
	RightLegColor  string
	IsTool         bool
	ToolID         int
	FaceID         int
	ShirtID        int
	PantsID        int
	TShirtID       int
	Hat1ID         int
	Hat2ID         int
	Hat3ID         int
	Hat4ID         int
	Hat5ID         int
	PreviewType    string
	PreviewTexture string
	PreviewObj     string
}

type RenderJob struct {
	Req    RenderRequest
	Result chan []byte
	Error  chan error
}

func Init() {
	files := []string{HeadFile, TorsoFile, LArmFile, RArmFile, LLegFile, RLegFile, TShirtFile}
	for _, f := range files {
		mesh, err := fauxgl.LoadOBJ(AssetPath + f)
		if err == nil {
			preloadmeshes[f] = mesh
		} else {
			log.Printf("mesh preload error for %s: %v", f, err)
		}
	}

	initBodyParts()

	go checkforload()

	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(i)
	}
}

func checkforload() {
	for {
		percent, err := cpu.Percent(time.Second, false)
		if err == nil && len(percent) > 0 {
			mutexll.Lock()
			getloaddd = percent[0]
			mutexll.Unlock()
		}
		time.Sleep(2 * time.Second)
	}
}

func gettheload() float64 {
	mutexll.RLock()
	defer mutexll.RUnlock()
	return getloaddd
}

func worker(id int) {
	for job := range jobq {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("panic in worker %d: %v", id, r)
					job.Error <- fmt.Errorf("render panic: %v", r)
				}
			}()
			data, err := renderav(job.Req)
			if err != nil {
				job.Error <- err
			} else {
				job.Result <- data
			}
		}()
	}
}

func submitRenderJob(req RenderRequest, timeoutMsg string) ([]byte, error) {
	for gettheload() >= loadThreshold {
		time.Sleep(time.Second)
	}

	resultChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	jobq <- RenderJob{
		Req:    req,
		Result: resultChan,
		Error:  errChan,
	}

	timer := time.NewTimer(renderTimeout)
	defer timer.Stop()
	select {
	case imgBytes := <-resultChan:
		return imgBytes, nil
	case renderErr := <-errChan:
		return nil, renderErr
	case <-timer.C:
		return nil, errors.New(timeoutMsg)
	}
}

func RenderUser(db *sql.DB, userID int) ([]byte, error) {
	return RenderUserWithPreviewType(db, userID, "")
}

func RenderUserHeadshot(db *sql.DB, userID int) ([]byte, error) {
	return RenderUserWithPreviewType(db, userID, "faces")
}

func RenderUserWithPreviewType(db *sql.DB, userID int, previewType string) ([]byte, error) {
	if db == nil {
		return nil, errors.New("database is not connected")
	}

	var headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor string
	var hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face int

	err := db.QueryRow(
		"SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color, hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face FROM avatar WHERE id = ?",
		userID,
	).Scan(
		&headColor, &larmColor, &rarmColor, &torsoColor, &llegColor, &rlegColor,
		&hat1, &hat2, &hat3, &hat4, &hat5, &tool, &shirt, &tshirt, &pants, &face,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			headColor = models.DefaultHeadColor
			larmColor = models.DefaultArmColor
			rarmColor = models.DefaultArmColor
			torsoColor = models.DefaultTorsoColor
			llegColor = models.DefaultLegColor
			rlegColor = models.DefaultLegColor
		} else {
			return nil, err
		}
	}

	req := RenderRequest{
		HeadColor:     headColor,
		TorsoColor:    torsoColor,
		LeftArmColor:  larmColor,
		RightArmColor: rarmColor,
		LeftLegColor:  llegColor,
		RightLegColor: rlegColor,
		IsTool:        tool > 0,
		ToolID:        tool,
		FaceID:        face,
		ShirtID:       shirt,
		PantsID:       pants,
		TShirtID:      tshirt,
		Hat1ID:        hat1,
		Hat2ID:        hat2,
		Hat3ID:        hat3,
		Hat4ID:        hat4,
		Hat5ID:        hat5,
		PreviewType:   previewType,
	}

	return submitRenderJob(req, "render timeout")
}

func RenderShopItem(itemType string, itemID int) ([]byte, error) {
	req := RenderRequest{
		HeadColor:     "f3b700",
		TorsoColor:    "0000ff",
		LeftArmColor:  "f3b700",
		RightArmColor: "f3b700",
		LeftLegColor:  "a4bd47",
		RightLegColor: "a4bd47",
		PreviewType:   itemType,
	}

	switch itemType {
	case "hat":
		req.Hat1ID = itemID
	case "shirts":
		req.ShirtID = itemID
	case "pants":
		req.PantsID = itemID
	case "tshirts":
		req.TShirtID = itemID
	case "faces":
		req.FaceID = itemID
	case "gear":
		req.IsTool = true
		req.ToolID = itemID
	}

	return submitRenderJob(req, "shop render timeout")
}

func RenderCreatePreview(category string, filePath string, texturePath string) ([]byte, error) {
	req := RenderRequest{
		HeadColor:     "f3b700",
		TorsoColor:    "0000ff",
		LeftArmColor:  "f3b700",
		RightArmColor: "f3b700",
		LeftLegColor:  "a4bd47",
		RightLegColor: "a4bd47",
	}

	switch category {
	case "hat", "face":
		req.PreviewType = "hat"
		req.PreviewObj = filePath
		req.PreviewTexture = texturePath
	case "tool":
		req.PreviewType = "gear"
		req.PreviewObj = filePath
		req.IsTool = true
		req.PreviewTexture = texturePath
	case "shirt":
		req.PreviewType = "shirts"
		req.PreviewTexture = filePath
	case "tshirt":
		req.PreviewType = "tshirts"
		req.PreviewTexture = filePath
	case "pants":
		req.PreviewType = "pants"
		req.PreviewTexture = filePath
	}

	return submitRenderJob(req, "preview render timeout")
}

func RenderOutfit(db *sql.DB, outfitID int) ([]byte, error) {
	if db == nil {
		return nil, errors.New("database is not connected")
	}

	var headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor string
	var hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face int

	err := db.QueryRow(
		"SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color, hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face FROM outfits WHERE id = ?",
		outfitID,
	).Scan(
		&headColor, &larmColor, &rarmColor, &torsoColor, &llegColor, &rlegColor,
		&hat1, &hat2, &hat3, &hat4, &hat5, &tool, &shirt, &tshirt, &pants, &face,
	)
	if err != nil {
		return nil, err
	}

	req := RenderRequest{
		HeadColor:     headColor,
		TorsoColor:    torsoColor,
		LeftArmColor:  larmColor,
		RightArmColor: rarmColor,
		LeftLegColor:  llegColor,
		RightLegColor: rlegColor,
		IsTool:        tool > 0,
		ToolID:        tool,
		FaceID:        face,
		ShirtID:       shirt,
		PantsID:       pants,
		TShirtID:      tshirt,
		Hat1ID:        hat1,
		Hat2ID:        hat2,
		Hat3ID:        hat3,
		Hat4ID:        hat4,
		Hat5ID:        hat5,
	}

	return submitRenderJob(req, "outfit render timeout")
}
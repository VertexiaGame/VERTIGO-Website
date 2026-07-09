package renderer

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fogleman/fauxgl"
	"github.com/shirou/gopsutil/v3/cpu"
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
	BaseFOV        = 55.0
	DefaultZoom    = 1.05
	AutoZoomMargin = 1.20
	AssetPath      = "./assets/char/"
	FacePath       = "./assets/faces/"
	ShirtPath      = "./assets/shirts/"
	PantsPath      = "./assets/pants/"
	TShirtPath     = "./assets/tshirts/"
	HatPath        = "./assets/hats/"
	HeadFile       = "Head.obj"
	TorsoFile      = "Torso.obj"
	LArmFile       = "LeftArm.obj"
	RArmFile       = "RightArm.obj"
	LLegFile       = "LeftLeg.obj"
	RLegFile       = "RightLeg.obj"
	TShirtFile     = "TShirt.obj"
)

var (
	preloadmeshes = make(map[string]*fauxgl.Mesh)
	getheadcenter fauxgl.Vector
	jobq          = make(chan RenderJob, 100)
	getloaddd     float64
	mutexll       sync.RWMutex
	bsurl         = "https://vertexia.xyz"
	httpClient    = &http.Client{Timeout: 15 * time.Second}
	downloadMutex sync.Mutex
	fileLocks     = make(map[string]*sync.Mutex)
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

type CompositeTexture struct {
	Layers []fauxgl.Texture
	Color  fauxgl.Color
}

func (t CompositeTexture) Sample(u, v float64) fauxgl.Color {
	final := t.Color
	final.A = 1.0
	for _, l := range t.Layers {
		if l == nil {
			continue
		}
		c := l.Sample(u, v)
		invA := 1.0 - c.A
		final.R = c.R*c.A + final.R*invA
		final.G = c.G*c.A + final.G*invA
		final.B = c.B*c.A + final.B*invA
	}
	return final
}

func (t CompositeTexture) BilinearSample(u, v float64) fauxgl.Color {
	final := t.Color
	final.A = 1.0
	for _, l := range t.Layers {
		if l == nil {
			continue
		}
		c := l.BilinearSample(u, v)
		invA := 1.0 - c.A
		final.R = c.R*c.A + final.R*invA
		final.G = c.G*c.A + final.G*invA
		final.B = c.B*c.A + final.B*invA
	}
	return final
}

func getFileLock(path string) *sync.Mutex {
	downloadMutex.Lock()
	defer downloadMutex.Unlock()
	if lock, exists := fileLocks[path]; exists {
		return lock
	}
	lock := &sync.Mutex{}
	fileLocks[path] = lock
	return lock
}

func downloadFile(url string, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status code error: %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchTexture(itemType string, id int, fallback string) (fauxgl.Texture, error) {
	if id <= 0 {
		if fallback != "" {
			return fauxgl.LoadTexture(fallback)
		}
		return nil, fmt.Errorf("invalid id")
	}
	url := fmt.Sprintf("%s/assets/uploads/shop/%s/%d.png?v=%d", bsurl, itemType, id, time.Now().UnixNano())
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d.png", itemType, id))

	lock := getFileLock(tmpPath)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		if err := downloadFile(url, tmpPath); err != nil {
			os.Remove(tmpPath)
			if fallback != "" {
				return fauxgl.LoadTexture(fallback)
			}
			return nil, err
		}
	}

	tex, err := fauxgl.LoadTexture(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		if fallback != "" {
			return fauxgl.LoadTexture(fallback)
		}
	}
	return tex, err
}

func fetchMesh(itemType string, id int) (*fauxgl.Mesh, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid id")
	}
	url := fmt.Sprintf("%s/assets/uploads/shop/%s/%d.obj?v=%d", bsurl, itemType, id, time.Now().UnixNano())
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%d.obj", itemType, id))

	lock := getFileLock(tmpPath)
	lock.Lock()
	defer lock.Unlock()

	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		if err := downloadFile(url, tmpPath); err != nil {
			os.Remove(tmpPath)
			return nil, err
		}
	}

	mesh, err := fauxgl.LoadOBJ(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
	}
	return mesh, err
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

	if meshhead, ok := preloadmeshes[HeadFile]; ok {
		getheadcenter = meshhead.BoundingBox().Center()
	}

	go checkforload()

	nwork := runtime.NumCPU()
	for i := 0; i < nwork; i++ {
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

func parsec(hexst string) fauxgl.Color {
	if len(hexst) > 0 && hexst[0] == '#' {
		hexst = hexst[1:]
	}
	c := fauxgl.Color{R: 1, G: 1, B: 1, A: 1}
	if len(hexst) == 6 {
		bytes, err := hex.DecodeString(hexst)
		if err == nil {
			c.R = float64(bytes[0]) / 255.0
			c.G = float64(bytes[1]) / 255.0
			c.B = float64(bytes[2]) / 255.0
		}
	}
	return c
}

func downsample(src image.Image, scale int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx()/scale, bounds.Dy()/scale
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a uint32
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					pr, pg, pb, pa := src.At(x*scale+sx, y*scale+sy).RGBA()
					r += pr
					g += pg
					b += pb
					a += pa
				}
			}
			area := uint32(scale * scale)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8((r / area) >> 8),
				G: uint8((g / area) >> 8),
				B: uint8((b / area) >> 8),
				A: uint8((a / area) >> 8),
			})
		}
	}
	return dst
}

func getNDC(m fauxgl.Matrix, v fauxgl.Vector) (float64, float64, float64, float64) {
	x := m.X00*v.X + m.X01*v.Y + m.X02*v.Z + m.X03
	y := m.X10*v.X + m.X11*v.Y + m.X12*v.Z + m.X13
	z := m.X20*v.X + m.X21*v.Y + m.X22*v.Z + m.X23
	w := m.X30*v.X + m.X31*v.Y + m.X32*v.Z + m.X33
	if w != 0 {
		return x / w, y / w, z / w, w
	}
	return x, y, z, w
}

func renderav(req RenderRequest) ([]byte, error) {
	context := fauxgl.NewContext(Width*ScaleFactor, Height*ScaleFactor)
	context.ClearColorBufferWith(fauxgl.Transparent)
	context.AlphaBlend = true

	mHead := fauxgl.Identity().
		Translate(getheadcenter.Negate()).
		Scale(fauxgl.Vector{X: 1, Y: 1, Z: 1}).
		Translate(fauxgl.Vector{X: 0, Y: 3, Z: 0})

	mTorso := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
	mRArm := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
	
	var mLArm fauxgl.Matrix
	if req.IsTool {
		if MeshOrig, Ok := preloadmeshes[LArmFile]; Ok {
			GeomCenter := MeshOrig.BoundingBox().Center()
			mLArm = fauxgl.Identity().
				Translate(GeomCenter.Negate()).
				Rotate(fauxgl.Vector{X: 1, Y: 0, Z: 0}, math.Pi/2).
				Translate(GeomCenter).
				Translate(fauxgl.Vector{X: -0, Y: -1, Z: 0.4})
		} else {
			mLArm = fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
		}
	} else {
		mLArm = fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
	}

	mLLeg := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
	mRLeg := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})
	mTShirt := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0.02})
	mTool := fauxgl.Translate(fauxgl.Vector{X: 0, Y: -1.5, Z: 0})

	var HatBox *fauxgl.Box
	var PreloadedHatMesh *fauxgl.Mesh

	if req.PreviewType == "hat" {
		if req.PreviewObj != "" {
			M, Err := fauxgl.LoadOBJ(req.PreviewObj)
			if Err == nil {
				PreloadedHatMesh = M
				MCopy := M.Copy()
				MCopy.Transform(mHead)
				B := MCopy.BoundingBox()
				HatBox = &B
			}
		} else if req.Hat1ID > 0 {
			M, Err := fetchMesh("hat", req.Hat1ID)
			if Err == nil {
				PreloadedHatMesh = M
				MCopy := M.Copy()
				MCopy.Transform(mHead)
				B := MCopy.BoundingBox()
				HatBox = &B
			}
		}
	}

	type LoadedHat struct {
		Mesh *fauxgl.Mesh
		Tex  fauxgl.Texture
		ID   int
	}
	var loadedHats []LoadedHat

	hats := []int{req.Hat1ID, req.Hat2ID, req.Hat3ID, req.Hat4ID, req.Hat5ID}
	for _, hid := range hats {
		if hid > 0 {
			var HatMesh *fauxgl.Mesh
			var Err error
			if req.PreviewType == "hat" && hid == req.Hat1ID && PreloadedHatMesh != nil {
				HatMesh = PreloadedHatMesh
			} else {
				HatMesh, Err = fetchMesh("hat", hid)
			}

			if Err != nil && HatMesh == nil {
				log.Printf("failed loading hat mesh %d: %v", hid, Err)
				continue
			}
			hatTex, err := fetchTexture("hat", hid, "")
			if err != nil {
				log.Printf("failed loading hat texture %d: %v", hid, err)
			} else {
				log.Printf("loaded hat texture %d", hid)
			}
			loadedHats = append(loadedHats, LoadedHat{Mesh: HatMesh, Tex: hatTex, ID: hid})
		}
	}

	var toolMesh *fauxgl.Mesh
	var toolTex fauxgl.Texture
	if req.PreviewType == "gear" && req.PreviewObj != "" {
		tm, err := fauxgl.LoadOBJ(req.PreviewObj)
		if err == nil {
			toolMesh = tm
			if req.PreviewTexture != "" {
				toolTex, _ = fauxgl.LoadTexture(req.PreviewTexture)
			}
		}
	} else if req.ToolID > 0 {
		tm, err := fetchMesh("gear", req.ToolID)
		if err == nil {
			toolMesh = tm
			toolTex, _ = fetchTexture("gear", req.ToolID, "")
		}
	}

	eye := fauxgl.Vector{X: CamPosX, Y: CamPosY, Z: CamPosZ}
	look := fauxgl.Vector{X: CamLookX, Y: CamLookY, Z: CamLookZ}
	up := fauxgl.Vector{X: 0, Y: 1, Z: 0}

	if req.IsTool {
		eye = fauxgl.Vector{X: 2, Y: 4.2, Z: 4.8}
	} else if req.PreviewType == "hat" && HatBox != nil {
		C := HatBox.Center()
		S := HatBox.Size()
		MaxS := math.Max(S.X, math.Max(S.Y, S.Z))
		look = fauxgl.Vector{X: C.X * 0.5, Y: C.Y - 1.0, Z: C.Z * 0.5}
		eye = fauxgl.Vector{X: C.X + 1.2 + MaxS*0.5, Y: C.Y + 0.5 + MaxS*0.2, Z: C.Z + 2.5 + MaxS*1.2}
	} else if req.PreviewType == "face" || req.PreviewType == "faces" {
		look = fauxgl.Vector{X: 0.0, Y: 3.0, Z: 0.0}
		eye = fauxgl.Vector{X: 1.0, Y: 3.2, Z: 2.5}
	}

	view := fauxgl.LookAt(eye, look, up)
	aspect := float64(Width) / float64(Height)

	allowAutoZoom := true
	if req.PreviewType == "face" || req.PreviewType == "faces" || req.PreviewType == "hat" {
		allowAutoZoom = false
	}

	defaultFov := 2 * math.Atan(math.Tan(BaseFOV*math.Pi/360.0)/DefaultZoom) * 180.0 / math.Pi
	var fov float64

	if allowAutoZoom {
		defaultProj := fauxgl.Perspective(defaultFov, aspect, 0.1, 1000)
		defaultMatrixcm := defaultProj.Mul(view)

		var baseCorners []fauxgl.Vector
		var attachmentCorners []fauxgl.Vector

		addBaseCorners := func(m *fauxgl.Mesh, mat fauxgl.Matrix) {
			if m == nil {
				return
			}
			b := m.BoundingBox()
			corners := []fauxgl.Vector{
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Min.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Min.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Max.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Max.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Min.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Min.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Max.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Max.Y, Z: b.Max.Z}),
			}
			baseCorners = append(baseCorners, corners...)
		}

		addAttachmentCorners := func(m *fauxgl.Mesh, mat fauxgl.Matrix) {
			if m == nil {
				return
			}
			b := m.BoundingBox()
			corners := []fauxgl.Vector{
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Min.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Min.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Max.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Min.X, Y: b.Max.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Min.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Min.Y, Z: b.Max.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Max.Y, Z: b.Min.Z}),
				mat.MulPosition(fauxgl.Vector{X: b.Max.X, Y: b.Max.Y, Z: b.Max.Z}),
			}
			attachmentCorners = append(attachmentCorners, corners...)
		}

		addBaseCorners(preloadmeshes[HeadFile], mHead)
		addBaseCorners(preloadmeshes[TorsoFile], mTorso)
		addBaseCorners(preloadmeshes[RArmFile], mRArm)
		addBaseCorners(preloadmeshes[LArmFile], mLArm)
		addBaseCorners(preloadmeshes[RLegFile], mRLeg)
		addBaseCorners(preloadmeshes[LLegFile], mLLeg)
		addBaseCorners(preloadmeshes[TShirtFile], mTShirt)

		for _, lh := range loadedHats {
			addAttachmentCorners(lh.Mesh, mHead)
		}

		if toolMesh != nil {
			addAttachmentCorners(toolMesh, mTool)
		}

		maxNDC := 0.0
		for _, p := range baseCorners {
			nx, ny, _, w := getNDC(defaultMatrixcm, p)
			if w <= 0.01 {
				continue
			}
			if math.Abs(nx) > maxNDC {
				maxNDC = math.Abs(nx)
			}
			if math.Abs(ny) > maxNDC {
				maxNDC = math.Abs(ny)
			}
		}

		for _, p := range attachmentCorners {
			nx, ny, _, w := getNDC(defaultMatrixcm, p)
			if w <= 0.01 {
				continue
			}
			if math.Abs(nx) > maxNDC {
				maxNDC = math.Abs(nx)
			}
			if math.Abs(ny) > maxNDC {
				maxNDC = math.Abs(ny)
			}
		}

		if maxNDC > 0.99 {
			factor := maxNDC * AutoZoomMargin
			newTan := (math.Tan(BaseFOV*math.Pi/360.0) / DefaultZoom) * factor
			fov = 2 * math.Atan(newTan) * 180.0 / math.Pi
		} else {
			fov = defaultFov
		}
	} else {
		fov = defaultFov
	}

	projection := fauxgl.Perspective(fov, aspect, 0.1, 1000)
	matrixcm := projection.Mul(view)

	var facetex fauxgl.Texture
	var shirttex fauxgl.Texture
	var pantstex fauxgl.Texture
	var tshirttex fauxgl.Texture

	var err error

	if (req.PreviewType == "face" || req.PreviewType == "faces") && req.PreviewTexture != "" {
		facetex, _ = fauxgl.LoadTexture(req.PreviewTexture)
	} else {
		facetex, _ = fetchTexture("faces", req.FaceID, FacePath+"0.png")
	}

	if (req.PreviewType == "shirt" || req.PreviewType == "shirts") && req.PreviewTexture != "" {
		shirttex, _ = fauxgl.LoadTexture(req.PreviewTexture)
	} else if req.ShirtID > 0 {
		shirttex, err = fetchTexture("shirts", req.ShirtID, "")
		if err != nil {
			log.Printf("failed loading shirt: %v", err)
		}
	}

	if (req.PreviewType == "pant" || req.PreviewType == "pants") && req.PreviewTexture != "" {
		pantstex, _ = fauxgl.LoadTexture(req.PreviewTexture)
	} else if req.PantsID > 0 {
		pantstex, err = fetchTexture("pants", req.PantsID, "")
		if err != nil {
			log.Printf("failed loading pants: %v", err)
		}
	}

	if (req.PreviewType == "tshirt" || req.PreviewType == "tshirts") && req.PreviewTexture != "" {
		tshirttex, _ = fauxgl.LoadTexture(req.PreviewTexture)
	} else if req.TShirtID > 0 {
		tshirttex, err = fetchTexture("tshirts", req.TShirtID, "")
		if err != nil {
			log.Printf("failed loading tshirt: %v", err)
		}
	}

	addpart := func(name string, colorHex string, matrix fauxgl.Matrix, texture fauxgl.Texture) {
		orig, ok := preloadmeshes[name]
		if !ok {
			log.Printf("mesh not found: %s", name)
			return
		}
		mesh := orig.Copy()
		c := parsec(colorHex)

		shader := fauxgl.NewPhongShader(matrixcm, fauxgl.Vector{X: -1, Y: 1, Z: 1}, eye)
		shader.AmbientColor = fauxgl.Color{R: Brightness, G: Brightness, B: Brightness, A: 1}
		shader.DiffuseColor = fauxgl.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}

		specularIntensity := 1.0 - Roughness
		if specularIntensity < 0 {
			specularIntensity = 0
		}
		shader.SpecularColor = fauxgl.Color{R: specularIntensity, G: specularIntensity, B: specularIntensity, A: 1}

		shader.ObjectColor = c
		shader.Texture = texture
		context.Shader = shader

		mesh.Transform(matrix)
		context.DrawMesh(mesh)
	}

	var headTex fauxgl.Texture
	if facetex != nil {
		headTex = CompositeTexture{Layers: []fauxgl.Texture{facetex}, Color: parsec(req.HeadColor)}
	}
	addpart(HeadFile, req.HeadColor, mHead, headTex)

	for _, lh := range loadedHats {
		hMesh := lh.Mesh.Copy()

		shader := fauxgl.NewPhongShader(matrixcm, fauxgl.Vector{X: -1, Y: 1, Z: 1}, eye)
		shader.AmbientColor = fauxgl.Color{R: Brightness, G: Brightness, B: Brightness, A: 1}
		shader.DiffuseColor = fauxgl.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}

		specularIntensity := 1.0 - Roughness
		if specularIntensity < 0 {
			specularIntensity = 0
		}
		shader.SpecularColor = fauxgl.Color{R: specularIntensity, G: specularIntensity, B: specularIntensity, A: 1}
		shader.ObjectColor = parsec("#FFFFFF")
		shader.Texture = lh.Tex
		context.Shader = shader

		hMesh.Transform(mHead)
		context.DrawMesh(hMesh)
	}

	if req.PreviewType == "hat" && req.PreviewObj != "" {
		if PreloadedHatMesh != nil {
			var hatTex fauxgl.Texture
			if req.PreviewTexture != "" {
				hatTex, _ = fauxgl.LoadTexture(req.PreviewTexture)
			}
			hMesh := PreloadedHatMesh.Copy()

			shader := fauxgl.NewPhongShader(matrixcm, fauxgl.Vector{X: -1, Y: 1, Z: 1}, eye)
			shader.AmbientColor = fauxgl.Color{R: Brightness, G: Brightness, B: Brightness, A: 1}
			shader.DiffuseColor = fauxgl.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}

			specularIntensity := 1.0 - Roughness
			if specularIntensity < 0 {
				specularIntensity = 0
			}
			shader.SpecularColor = fauxgl.Color{R: specularIntensity, G: specularIntensity, B: specularIntensity, A: 1}
			shader.ObjectColor = parsec("#FFFFFF")
			shader.Texture = hatTex
			context.Shader = shader

			hMesh.Transform(mHead)
			context.DrawMesh(hMesh)
		}
	}

	mTorsoLayers := []fauxgl.Texture{}
	if pantstex != nil {
		mTorsoLayers = append(mTorsoLayers, pantstex)
	}
	if shirttex != nil {
		mTorsoLayers = append(mTorsoLayers, shirttex)
	}
	var torsoTex fauxgl.Texture
	if len(mTorsoLayers) > 0 {
		torsoTex = CompositeTexture{Layers: mTorsoLayers, Color: parsec(req.TorsoColor)}
	}
	addpart(TorsoFile, req.TorsoColor, mTorso, torsoTex)

	var rArmTex fauxgl.Texture
	if shirttex != nil {
		rArmTex = CompositeTexture{Layers: []fauxgl.Texture{shirttex}, Color: parsec(req.RightArmColor)}
	}
	addpart(RArmFile, req.RightArmColor, mRArm, rArmTex)

	var lArmTex fauxgl.Texture
	if shirttex != nil {
		lArmTex = CompositeTexture{Layers: []fauxgl.Texture{shirttex}, Color: parsec(req.LeftArmColor)}
	}
	addpart(LArmFile, req.LeftArmColor, mLArm, lArmTex)

	var lLegTex fauxgl.Texture
	if pantstex != nil {
		lLegTex = CompositeTexture{Layers: []fauxgl.Texture{pantstex}, Color: parsec(req.LeftLegColor)}
	}
	addpart(LLegFile, req.LeftLegColor, mLLeg, lLegTex)

	var rLegTex fauxgl.Texture
	if pantstex != nil {
		rLegTex = CompositeTexture{Layers: []fauxgl.Texture{pantstex}, Color: parsec(req.RightLegColor)}
	}
	addpart(RLegFile, req.RightLegColor, mRLeg, rLegTex)

	if toolMesh != nil {
		tMesh := toolMesh.Copy()

		shader := fauxgl.NewPhongShader(matrixcm, fauxgl.Vector{X: -1, Y: 1, Z: 1}, eye)
		shader.AmbientColor = fauxgl.Color{R: Brightness, G: Brightness, B: Brightness, A: 1}
		shader.DiffuseColor = fauxgl.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}

		specularIntensity := 1.0 - Roughness
		if specularIntensity < 0 {
			specularIntensity = 0
		}
		shader.SpecularColor = fauxgl.Color{R: specularIntensity, G: specularIntensity, B: specularIntensity, A: 1}
		shader.ObjectColor = parsec("#FFFFFF")
		shader.Texture = toolTex
		context.Shader = shader

		tMesh.Transform(mTool)
		context.DrawMesh(tMesh)
	}

	if tshirttex != nil {
		addpart(TShirtFile, "#FFFFFF", mTShirt, tshirttex)
	}

	fullImg := context.Image()
	finalImg := downsample(fullImg, ScaleFactor)

	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func RenderUser(db *sql.DB, userID int) ([]byte, error) {
	return RenderUserWithPreviewType(db, userID, "")
}

func RenderUserHeadshot(db *sql.DB, userID int) ([]byte, error) {
	return RenderUserWithPreviewType(db, userID, "faces")
}

func RenderUserWithPreviewType(db *sql.DB, userID int, previewType string) ([]byte, error) {
	var headColor, larmColor, rarmColor, torsoColor, llegColor, rlegColor string
	var hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face int

	if db == nil {
		return nil, errors.New("database is not connected")
	}

	query := "SELECT head_color, larm_color, rarm_color, torso_color, lleg_color, rleg_color, hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face FROM avatar WHERE id = ?"
	err := db.QueryRow(query, userID).Scan(
		&headColor, &larmColor, &rarmColor, &torsoColor, &llegColor, &rlegColor,
		&hat1, &hat2, &hat3, &hat4, &hat5, &tool, &shirt, &tshirt, &pants, &face,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			headColor = "f3b700"
			larmColor = "f3b700"
			rarmColor = "f3b700"
			torsoColor = "c60000"
			llegColor = "650013"
			rlegColor = "650013"
			hat1, hat2, hat3, hat4, hat5, tool, shirt, tshirt, pants, face = 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
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

	for gettheload() >= 95.0 {
		time.Sleep(1 * time.Second)
	}

	resultChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	jobq <- RenderJob{
		Req:    req,
		Result: resultChan,
		Error:  errChan,
	}

	select {
	case imgBytes := <-resultChan:
		return imgBytes, nil
	case renderErr := <-errChan:
		return nil, renderErr
	case <-time.After(15 * time.Second):
		return nil, errors.New("render timeout")
	}
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

	for gettheload() >= 95.0 {
		time.Sleep(1 * time.Second)
	}

	resultChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	jobq <- RenderJob{
		Req:    req,
		Result: resultChan,
		Error:  errChan,
	}

	select {
	case imgBytes := <-resultChan:
		return imgBytes, nil
	case renderErr := <-errChan:
		return nil, renderErr
	case <-time.After(15 * time.Second):
		return nil, errors.New("shop render timeout")
	}
}
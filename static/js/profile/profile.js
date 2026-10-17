import * as THREE from 'three';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';

window.THREE = THREE;

const ROTATION_SPEED = 0.008;
const AUTO_ROTATE_SPEED = 0.005;
const MIN_PHI = -1.45;
const MAX_PHI = 1.45;
const MIN_RADIUS = 2.2;
const MAX_RADIUS = 12.0;

const clamp = (val, min, max) => Math.max(min, Math.min(max, val));

export class AvatarViewer {
    constructor(container, userId, options = {}) {
        this.container = container;
        this.userId = userId;
        this.dummy = !!options.dummy;
        this.toolPose = !!options.toolPose;

        this.target = new THREE.Vector3(0, 0.8, 0);
        this.spherical = {
            radius: 6.8,
            theta: -0.35,
            phi: 0.28
        };
        this.targetRadius = 6.8;

        this.isDragging = false;
        this.previousPointerPosition = { x: 0, y: 0 };
        this.initialPinchDistance = null;
        this.animationId = null;
        this.spinnerEl = null;
        this.autoRotate = true;

        this.initScene();
        this.initRenderer();
        this.initLights();
        this.initControls();
        this.initResizeObserver();
        this.showSpinner();
    }

    showSpinner() {
        this.spinnerEl = document.createElement('div');
        this.spinnerEl.className = 'musld';
        this.spinnerEl.style.position = 'absolute';
        this.spinnerEl.style.top = '50%';
        this.spinnerEl.style.left = '50%';
        this.spinnerEl.style.transform = 'translate(-50%, -50%)';
        this.spinnerEl.style.zIndex = '10';
        this.spinnerEl.style.pointerEvents = 'none';
        this.spinnerEl.style.margin = '0';
        this.spinnerEl.style.padding = '0';
        this.spinnerEl.innerHTML = `
            <svg class="dmspn" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
                <rect class="sq sq-1" x="43" y="7" width="14" height="14"/>
                <rect class="sq sq-2" x="25" y="25" width="14" height="14"/>
                <rect class="sq sq-3" x="7" y="43" width="14" height="14"/>
                <rect class="sq sq-4" x="25" y="61" width="14" height="14"/>
                <rect class="sq sq-5" x="43" y="79" width="14" height="14"/>
                <rect class="sq sq-6" x="61" y="61" width="14" height="14"/>
                <rect class="sq sq-7" x="79" y="43" width="14" height="14"/>
                <rect class="sq sq-8" x="61" y="25" width="14" height="14"/>
            </svg>
        `;
        this.container.appendChild(this.spinnerEl);
    }

    hideSpinner() {
        if (this.spinnerEl && this.spinnerEl.parentNode) {
            this.spinnerEl.parentNode.removeChild(this.spinnerEl);
            this.spinnerEl = null;
        }
    }

    initScene() {
        this.scene = new THREE.Scene();
        const width = this.container.clientWidth || 300;
        const height = this.container.clientHeight || 340;

        this.camera = new THREE.PerspectiveCamera(60, width / height, 0.1, 1000);
        this.updateCameraPosition();

        this.avatarGroup = new THREE.Group();
        this.scene.add(this.avatarGroup);

        const shadowPlaneGeo = new THREE.PlaneGeometry(12, 12);
        const shadowPlaneMat = new THREE.ShadowMaterial({ opacity: 0.15 });
        this.shadowPlane = new THREE.Mesh(shadowPlaneGeo, shadowPlaneMat);
        this.shadowPlane.rotation.x = -Math.PI / 2;
        this.shadowPlane.position.y = -2.2;
        this.shadowPlane.receiveShadow = true;
        this.scene.add(this.shadowPlane);
    }

    initRenderer() {
        this.container.innerHTML = '';
        const width = this.container.clientWidth || 300;
        const height = this.container.clientHeight || 340;

        this.renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        if (this.renderer.domElement && this.renderer.domElement.style) {
            this.renderer.domElement.style.touchAction = 'none';
            this.renderer.domElement.style.display = 'block';
        }

        if (THREE.NeutralToneMapping) {
            this.renderer.toneMapping = THREE.NeutralToneMapping;
        } else if (THREE.LinearToneMapping) {
            this.renderer.toneMapping = THREE.LinearToneMapping;
        }
        this.renderer.toneMappingExposure = 1.0;

        if (THREE.PCFShadowMap) {
            this.renderer.shadowMap.enabled = true;
            this.renderer.shadowMap.type = THREE.PCFShadowMap;
        } else if (THREE.PCFSoftShadowMap) {
            this.renderer.shadowMap.enabled = true;
            this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        }

        this.container.appendChild(this.renderer.domElement);
    }

    initLights() {
        const hemiLight = new THREE.HemisphereLight(0xffffff, 0x888888, 0.8);
        this.scene.add(hemiLight);

        const keyLight = new THREE.DirectionalLight(0xffffff, 1.0);
        keyLight.position.set(4, 7, 5);
        if (keyLight.shadow) {
            keyLight.castShadow = true;
            keyLight.shadow.mapSize.width = 1024;
            keyLight.shadow.mapSize.height = 1024;
            keyLight.shadow.camera.near = 0.5;
            keyLight.shadow.camera.far = 18;
            keyLight.shadow.bias = -0.0005;
        }
        this.scene.add(keyLight);

        const fillLight = new THREE.DirectionalLight(0xffffff, 0.3);
        fillLight.position.set(-4, 3, 3);
        this.scene.add(fillLight);

        const pointLight = new THREE.PointLight(0xffffff, 0.25, 10);
        pointLight.position.set(0, 2, 2);
        this.scene.add(pointLight);
    }

    initControls() {
        this.activePointers = new Map();

        this.boundPointerDown = this.onPointerDown.bind(this);
        this.boundPointerMove = this.onPointerMove.bind(this);
        this.boundPointerUp = this.onPointerUp.bind(this);
        this.boundWheel = this.onWheel.bind(this);
        this.boundContextMenu = e => e.preventDefault();

        if (this.container && this.container.style) {
            this.container.style.touchAction = 'none';
        }
        if (this.renderer && this.renderer.domElement && this.renderer.domElement.style) {
            this.renderer.domElement.style.touchAction = 'none';
        }

        this.container.addEventListener('pointerdown', this.boundPointerDown, { passive: false });
        this.container.addEventListener('contextmenu', this.boundContextMenu);
        window.addEventListener('pointermove', this.boundPointerMove, { passive: false });
        window.addEventListener('pointerup', this.boundPointerUp);
        window.addEventListener('pointercancel', this.boundPointerUp);
        this.container.addEventListener('wheel', this.boundWheel, { passive: false });
    }

    removeEventListeners() {
        if (this.container) {
            this.container.removeEventListener('pointerdown', this.boundPointerDown);
            this.container.removeEventListener('contextmenu', this.boundContextMenu);
            this.container.removeEventListener('wheel', this.boundWheel);
        }
        window.removeEventListener('pointermove', this.boundPointerMove);
        window.removeEventListener('pointerup', this.boundPointerUp);
        window.removeEventListener('pointercancel', this.boundPointerUp);
    }

    initResizeObserver() {
        if (window.ResizeObserver) {
            this.resizeObserver = new ResizeObserver(entries => {
                for (let entry of entries) {
                    const w = entry.contentRect.width;
                    const h = entry.contentRect.height;
                    if (w > 0 && h > 0 && this.renderer && this.camera) {
                        this.camera.aspect = w / h;
                        this.camera.updateProjectionMatrix();
                        this.renderer.setSize(w, h);
                    }
                }
            });
            this.resizeObserver.observe(this.container);
        }
    }

    rotateCamera(deltaX, deltaY) {
        this.spherical.theta -= deltaX * ROTATION_SPEED;
        this.spherical.phi += deltaY * ROTATION_SPEED;
        this.spherical.phi = clamp(this.spherical.phi, MIN_PHI, MAX_PHI);
    }

    zoomCamera(delta) {
        this.targetRadius += delta;
        this.targetRadius = clamp(this.targetRadius, MIN_RADIUS, MAX_RADIUS);
    }

    updateCameraPosition() {
        if (this.autoRotate && !this.isDragging && (!this.activePointers || this.activePointers.size === 0)) {
            this.spherical.theta += AUTO_ROTATE_SPEED;
        }

        this.spherical.radius += (this.targetRadius - this.spherical.radius) * 0.1;

        const x = this.target.x + this.spherical.radius * Math.sin(this.spherical.theta) * Math.cos(this.spherical.phi);
        const y = this.target.y + this.spherical.radius * Math.sin(this.spherical.phi);
        const z = this.target.z + this.spherical.radius * Math.cos(this.spherical.theta) * Math.cos(this.spherical.phi);

        this.camera.position.set(x, y, z);
        this.camera.lookAt(this.target);
    }

    onPointerDown(e) {
        if (e.cancelable && e.pointerType === 'touch') {
            try { e.preventDefault(); } catch {}
        }
        try { this.container.setPointerCapture(e.pointerId); } catch {}
        this.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
        if (this.activePointers.size === 1) {
            this.isDragging = true;
            this.previousPointerPosition = { x: e.clientX, y: e.clientY };
        } else if (this.activePointers.size === 2) {
            this.isDragging = false;
            const pts = Array.from(this.activePointers.values());
            this.initialPinchDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
        }
    }

    onPointerMove(e) {
        if (!this.activePointers.has(e.pointerId)) return;
        if (e.cancelable && e.pointerType === 'touch' && this.activePointers.size > 0) {
            try { e.preventDefault(); } catch {}
        }
        this.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });

        if (this.activePointers.size === 1 && this.isDragging) {
            const deltaX = e.clientX - this.previousPointerPosition.x;
            const deltaY = e.clientY - this.previousPointerPosition.y;

            this.rotateCamera(deltaX, deltaY);
            this.previousPointerPosition = { x: e.clientX, y: e.clientY };
        } else if (this.activePointers.size === 2 && this.initialPinchDistance !== null) {
            const pts = Array.from(this.activePointers.values());
            const currentDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
            if (currentDistance > 0) {
                const deltaDistance = this.initialPinchDistance - currentDistance;
                this.zoomCamera(deltaDistance * 0.015);
                this.initialPinchDistance = currentDistance;
            }
        }
    }

    onPointerUp(e) {
        this.activePointers.delete(e.pointerId);
        if (this.activePointers.size < 2) {
            this.initialPinchDistance = null;
        }
        if (this.activePointers.size === 1) {
            this.isDragging = true;
            const p = Array.from(this.activePointers.values())[0];
            this.previousPointerPosition = { x: p.x, y: p.y };
        } else if (this.activePointers.size === 0) {
            this.isDragging = false;
        }
    }

    onWheel(e) {
        e.preventDefault();
        this.zoomCamera(e.deltaY * 0.005);
    }

    async createHeadTexture(skinColor, faceUrl) {
        return new Promise(resolve => {
            const canvas = document.createElement('canvas');
            canvas.width = 512;
            canvas.height = 512;
            const ctx = canvas.getContext('2d');

            ctx.fillStyle = skinColor;
            ctx.fillRect(0, 0, 512, 512);

            const finishTexture = () => {
                const tex = new THREE.CanvasTexture(canvas);
                if (THREE.SRGBColorSpace) {
                    tex.colorSpace = THREE.SRGBColorSpace;
                }
                resolve(tex);
            };

            if (!faceUrl) {
                finishTexture();
                return;
            }

            const img = new Image();
            img.crossOrigin = 'anonymous';
            img.onload = () => {
                ctx.drawImage(img, 0, 0, 512, 512);
                finishTexture();
            };
            img.onerror = () => {
                finishTexture();
            };
            img.src = faceUrl;
        });
    }

    async fetchAvatarData() {
        try {
            const response = await fetch(`/api/v1/avatar/data/${this.userId}`);
            if (!response.ok) throw new Error('Failed to load avatar data');
            return await response.json();
        } catch {
            return null;
        }
    }

    preloadImage(url) {
        return new Promise((resolve, reject) => {
            const img = new Image();
            img.onload = () => resolve(url);
            img.onerror = () => reject(new Error('missing image'));
            img.src = url;
        });
    }

    async resolveFaceUrl(faceId) {
        const urls = [`/assets/uploads/shop/faces/${faceId}.png`, `/assets/faces/${faceId}.png`];
        for (const url of urls) {
            try {
                return await this.preloadImage(url);
            } catch {}
        }
        return null;
    }

    async fetchItemTransform(id) {
        try {
            const response = await fetch(`/api/v1/shop/items/${id}`);
            if (!response.ok) return null;
            const detail = await response.json();
            const raw = detail ? detail.transform : null;
            if (!raw) return null;
            const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw;
            if (!parsed || typeof parsed !== 'object') return null;
            return parsed;
        } catch {
            return null;
        }
    }

    hasStoredPosition(transform) {
        if (!transform || typeof transform !== 'object') return false;
        const cand = (transform.placement && transform.placement.position) || transform.position;
        if (!cand) return false;
        return [cand.x, cand.y, cand.z].every(n => typeof n === 'number' && isFinite(n));
    }

    poseToolArm(obj) {
        obj.updateMatrixWorld(true);
        const c = new THREE.Box3().setFromObject(obj).getCenter(new THREE.Vector3());
        obj.traverse(child => {
            if (child.isMesh && child.geometry) {
                child.geometry.translate(-c.x, -c.y, -c.z);
                child.geometry.rotateX(-Math.PI / 2);
                child.geometry.translate(c.x, c.y + 0.5, c.z + 0.4);
            }
        });
    }

    loadClothTexture(textureLoader, type, id, tex) {
        return new Promise(resolve => {
            if (!id || id <= 0) {
                resolve(tex || null);
                return;
            }
            const urls = [`/assets/uploads/shop/${type}/${id}.png`, `/assets/shop/${type}/${id}.png`];
            const tryNext = index => {
                if (index >= urls.length) {
                    resolve(tex || null);
                    return;
                }
                const loaded = textureLoader.load(urls[index], () => resolve(loaded), undefined, () => tryNext(index + 1));
                if (loaded) loaded.colorSpace = THREE.SRGBColorSpace;
            };
            tryNext(0);
        });
    }

    async renderAvatar(data) {
        const faceId = data ? data.face_id : 0;
        const faceUrl = await this.resolveFaceUrl(faceId);

        const textureLoader = new THREE.TextureLoader();
        const shirtTex = await this.loadClothTexture(textureLoader, 'shirts', data ? data.shirt : 0);
        const pantsTex = await this.loadClothTexture(textureLoader, 'pants', data ? data.pants : 0);
        const tshirtTex = await this.loadClothTexture(textureLoader, 'tshirts', data ? data.tshirt : 0);

        const partTexture = name => {
            if (name === 'Torso' || name === 'LeftArm' || name === 'RightArm') return shirtTex;
            if (name === 'LeftLeg' || name === 'RightLeg') return pantsTex;
            return null;
        };

        const bodyParts = [
            { name: 'Head', file: 'Head.obj', color: data ? data.head_color : '#f3b700', isHead: true },
            { name: 'Torso', file: 'Torso.obj', color: data ? data.torso_color : '#c60000' },
            { name: 'LeftArm', file: 'LeftArm.obj', color: data ? data.larm_color : '#f3b700' },
            { name: 'RightArm', file: 'RightArm.obj', color: data ? data.rarm_color : '#f3b700' },
            { name: 'LeftLeg', file: 'LeftLeg.obj', color: data ? data.lleg_color : '#650013' },
            { name: 'RightLeg', file: 'RightLeg.obj', color: data ? data.rleg_color : '#650013' }
        ];

        const loader = new OBJLoader();

        const partPromises = bodyParts.map(async part => {
            try {
                const obj = await loader.loadAsync(`/assets/char/${part.file}`);
                return { part, obj };
            } catch {
                return { part, obj: null };
            }
        });

        const results = await Promise.all(partPromises);

        let headObj = null;
        let leftArmObj = null;
        for (const { part, obj } of results) {
            if (!obj) continue;

            let material;
            if (part.isHead) {
                const headTex = await this.createHeadTexture(part.color, faceUrl);
                material = new THREE.MeshStandardMaterial({
                    map: headTex,
                    roughness: 0.6,
                    metalness: 0.0,
                    emissive: new THREE.Color(0xffffff),
                    emissiveMap: headTex,
                    emissiveIntensity: 0.2
                });
            } else {
                const clothTex = partTexture(part.name);
                if (clothTex) {
                    material = new THREE.MeshStandardMaterial({
                        map: clothTex,
                        color: new THREE.Color(part.color),
                        roughness: 0.6,
                        metalness: 0.0
                    });
                } else {
                    material = new THREE.MeshStandardMaterial({
                        color: new THREE.Color(part.color),
                        roughness: 0.6,
                        metalness: 0.0,
                        emissive: new THREE.Color(part.color),
                        emissiveIntensity: 0.2
                    });
                }
            }

            obj.traverse(child => {
                if (child.isMesh) {
                    child.material = material;
                    child.castShadow = true;
                    child.receiveShadow = true;
                }
            });

            if (part.isHead) headObj = obj;
            if (part.name === 'LeftArm') leftArmObj = obj;
            this.avatarGroup.add(obj);
        }

        if (tshirtTex) {
            try {
                const tshirtObj = await loader.loadAsync('/assets/char/TShirt.obj');
                if (tshirtObj) {
                    const tshirtMat = new THREE.MeshStandardMaterial({
                        map: tshirtTex,
                        color: new THREE.Color(0xffffff),
                        roughness: 0.6,
                        metalness: 0.0
                    });
                    tshirtObj.traverse(child => {
                        if (child.isMesh) {
                            child.material = tshirtMat;
                            child.castShadow = true;
                            child.receiveShadow = true;
                        }
                    });
                    this.avatarGroup.add(tshirtObj);
                }
            } catch {}
        }

        if (data && data.tool > 0 && leftArmObj) {
            this.poseToolArm(leftArmObj);
        }

        const hats = data ? [data.hat1, data.hat2, data.hat3, data.hat4, data.hat5].filter(id => id && id > 0) : [];
        if (hats.length > 0 || (data && data.tool > 0)) {
            const loadShopTexture = (type, id) => new Promise(resolve => {
                const urls = [`/assets/uploads/shop/${type}/${id}.png`, `/assets/shop/${type}/${id}.png`];
                const tryNext = index => {
                    if (index >= urls.length) {
                        resolve(null);
                        return;
                    }
                    const tex = textureLoader.load(urls[index], () => resolve(tex), undefined, () => tryNext(index + 1));
                    if (tex) tex.colorSpace = THREE.SRGBColorSpace;
                };
                tryNext(0);
            });
            const loadHatTexture = id => loadShopTexture('hat', id);
            const loadToolTexture = id => loadShopTexture('gear', id);
            for (const hatId of hats) {
                try {
                    let hatObj = null;
                    const hatUrls = [`/assets/uploads/shop/hat/${hatId}.obj`, `/assets/shop/hat/${hatId}.obj`];
                    for (const hatUrl of hatUrls) {
                        try {
                            hatObj = await loader.loadAsync(hatUrl);
                            if (hatObj) break;
                        } catch {}
                    }
                    if (!hatObj) continue;
                    const hatTransform = await this.fetchItemTransform(hatId);
                    const hatPositioned = this.hasStoredPosition(hatTransform);
                    const hatTex = await loadHatTexture(hatId);
                    const hatMat = new THREE.MeshStandardMaterial({
                        color: hatTex ? new THREE.Color(0xffffff) : new THREE.Color(0xd6d6d6),
                        map: hatTex || null,
                        roughness: 0.6,
                        metalness: 0.0
                    });
                    hatObj.traverse(child => {
                        if (child.isMesh) {
                            child.material = hatMat;
                            child.castShadow = true;
                            child.receiveShadow = true;
                            if (child.geometry && !child.geometry.attributes.normal) child.geometry.computeVertexNormals();
                        }
                    });
                    if (!hatPositioned && headObj) {
                        const headBox = new THREE.Box3().setFromObject(headObj);
                        const headCenter = headBox.getCenter(new THREE.Vector3());
                        const hatBox = new THREE.Box3().setFromObject(hatObj);
                        const hatCenter = hatBox.getCenter(new THREE.Vector3());
                        hatObj.position.sub(hatCenter).add(headCenter);
                        hatObj.position.y += headBox.getSize(new THREE.Vector3()).y * 0.45;
                    }
                    this.avatarGroup.add(hatObj);
                } catch {}
            }
            if (data && data.tool > 0) {
                try {
                    let toolObj = null;
                    const toolUrls = [`/assets/uploads/shop/gear/${data.tool}.obj`, `/assets/shop/gear/${data.tool}.obj`];
                    for (const toolUrl of toolUrls) {
                        try {
                            toolObj = await loader.loadAsync(toolUrl);
                            if (toolObj) break;
                        } catch {}
                    }
                    if (toolObj) {
                        const toolTransform = await this.fetchItemTransform(data.tool);
                        const toolPositioned = this.hasStoredPosition(toolTransform);
                        const toolTex = await loadToolTexture(data.tool);
                        const toolMat = new THREE.MeshStandardMaterial({
                            color: toolTex ? new THREE.Color(0xffffff) : new THREE.Color(0xd6d6d6),
                            map: toolTex || null,
                            roughness: 0.6,
                            metalness: 0.0
                        });
                        toolObj.traverse(child => {
                            if (child.isMesh) {
                                child.material = toolMat;
                                child.castShadow = true;
                                child.receiveShadow = true;
                                if (child.geometry && !child.geometry.attributes.normal) child.geometry.computeVertexNormals();
                            }
                        });
                        if (!toolPositioned) {
                            toolObj.position.set(0.6, 0.2, 0.3);
                            toolObj.rotation.set(0, 0, Math.PI / 2);
                        }
                        this.avatarGroup.add(toolObj);
                    }
                } catch {}
            }
        }

        const box = new THREE.Box3().setFromObject(this.avatarGroup);
        if (!box.isEmpty()) {
            const center = box.getCenter(new THREE.Vector3());
            if (!isNaN(center.x) && !isNaN(center.y) && !isNaN(center.z)) {
                this.avatarGroup.position.x = -center.x;
                this.avatarGroup.position.y = -center.y + 0.1;
                this.avatarGroup.position.z = -center.z;
                this.shadowPlane.position.y = box.min.y - center.y + 0.1;
            }
        }
    }

    async renderDummyAvatar(toolPose = false) {
        const grey = '#B3B3B3';
        const faceUrl = await this.resolveFaceUrl(0);
        const bodyParts = [
            { name: 'Head', file: 'Head.obj', color: grey, isHead: true },
            { name: 'Torso', file: 'Torso.obj', color: grey },
            { name: 'LeftArm', file: 'LeftArm.obj', color: grey },
            { name: 'RightArm', file: 'RightArm.obj', color: grey },
            { name: 'LeftLeg', file: 'LeftLeg.obj', color: grey },
            { name: 'RightLeg', file: 'RightLeg.obj', color: grey }
        ];
        const loader = new OBJLoader();
        const partPromises = bodyParts.map(async part => {
            try {
                const obj = await loader.loadAsync(`/assets/char/${part.file}`);
                return { part, obj };
            } catch {
                return { part, obj: null };
            }
        });
        const results = await Promise.all(partPromises);
        let leftArmObj = null;
        for (const { part, obj } of results) {
            if (!obj) continue;
            let material;
            if (part.isHead) {
                const headTex = await this.createHeadTexture(part.color, faceUrl);
                material = new THREE.MeshStandardMaterial({
                    map: headTex,
                    roughness: 0.6,
                    metalness: 0.0,
                    emissive: new THREE.Color(0xffffff),
                    emissiveMap: headTex,
                    emissiveIntensity: 0.2
                });
            } else {
                material = new THREE.MeshStandardMaterial({
                    color: new THREE.Color(part.color),
                    roughness: 0.6,
                    metalness: 0.0,
                    emissive: new THREE.Color(part.color),
                    emissiveIntensity: 0.2
                });
            }
            obj.traverse(child => {
                if (child.isMesh) {
                    child.material = material;
                    child.castShadow = true;
                    child.receiveShadow = true;
                }
            });
            if (part.name === 'LeftArm') leftArmObj = obj;
            this.avatarGroup.add(obj);
        }
        if (toolPose && leftArmObj) {
            this.poseToolArm(leftArmObj);
        }
        const box = new THREE.Box3().setFromObject(this.avatarGroup);
        if (!box.isEmpty()) {
            const center = box.getCenter(new THREE.Vector3());
            if (!isNaN(center.x) && !isNaN(center.y) && !isNaN(center.z)) {
                this.avatarGroup.position.x = -center.x;
                this.avatarGroup.position.y = -center.y + 0.1;
                this.avatarGroup.position.z = -center.z;
                this.shadowPlane.position.y = box.min.y - center.y + 0.1;
            }
        }
    }

    async load() {
        if (this.dummy) {
            await this.renderDummyAvatar(this.toolPose);
        } else {
            const data = await this.fetchAvatarData();
            await this.renderAvatar(data);
        }
        this.hideSpinner();
        this.animate();
    }

    animate() {
        this.animationId = requestAnimationFrame(() => this.animate());
        this.updateCameraPosition();
        if (this.renderer && this.scene && this.camera) {
            this.renderer.render(this.scene, this.camera);
        }
    }

    disposeScene() {
        if (this.scene) {
            this.scene.traverse(obj => {
                if (obj.geometry) {
                    obj.geometry.dispose();
                }
                if (obj.material) {
                    if (Array.isArray(obj.material)) {
                        obj.material.forEach(m => {
                            if (m.map) m.map.dispose();
                            m.dispose();
                        });
                    } else {
                        if (obj.material.map) obj.material.map.dispose();
                        obj.material.dispose();
                    }
                }
            });
        }
        if (this.renderer) {
            if (this.renderer.domElement && this.renderer.domElement.parentNode) {
                this.renderer.domElement.parentNode.removeChild(this.renderer.domElement);
            }
            this.renderer.dispose();
        }
    }

    destroy() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
            this.resizeObserver = null;
        }
        this.hideSpinner();
        this.removeEventListeners();
        this.disposeScene();
    }
}

let activeAvatarViewer = null;

window.init3DAvatar = function(userId) {
    if (activeAvatarViewer) {
        activeAvatarViewer.destroy();
        activeAvatarViewer = null;
    }

    const container = document.getElementById('avatar3d');
    if (!container) return;

    activeAvatarViewer = new AvatarViewer(container, userId);
    activeAvatarViewer.load();
};

window.scrollWearing = function(dir) {
    const row = document.getElementById('prfWearingRow');
    if (!row) return;
    row.scrollBy({ left: dir * Math.max(120, row.clientWidth * 0.8), behavior: 'smooth' });
};

window.updateWearingArrows = function() {
    const row = document.getElementById('prfWearingRow');
    const left = document.getElementById('prfWearingLeft');
    const right = document.getElementById('prfWearingRight');
    if (!row || !left || !right) return;
    const fits = row.scrollWidth <= row.clientWidth + 1;
    left.style.display = fits ? 'none' : '';
    right.style.display = fits ? 'none' : '';
    if (!fits) {
        left.style.opacity = row.scrollLeft <= 1 ? '0.35' : '';
        right.style.opacity = row.scrollLeft >= row.scrollWidth - row.clientWidth - 1 ? '0.35' : '';
    }
};

const initProfile = () => {
    if (typeof window.updateProfileTabIndicator === 'function') {
        setTimeout(window.updateProfileTabIndicator, 50);
        setTimeout(window.updateProfileTabIndicator, 150);
        setTimeout(window.updateProfileTabIndicator, 350);
    }

    if (typeof window.updateWearingArrows === 'function') {
        window.updateWearingArrows();
        setTimeout(() => window.updateWearingArrows(), 300);
    }

    const row = document.getElementById('prfWearingRow');
    if (row && !row.dataset.arrowsBound) {
        row.dataset.arrowsBound = '1';
        row.addEventListener('scroll', () => window.updateWearingArrows(), { passive: true });
    }

    const bodyFig = document.querySelector('.prfbodyfig');
    if (bodyFig && !bodyFig.dataset.tipBound) {
        bodyFig.dataset.tipBound = '1';
        const bodyTip = document.getElementById('prfBodyTip');
        bodyFig.addEventListener('mouseleave', () => bodyFig.classList.remove('showtip'));
        bodyFig.querySelectorAll('[data-tip]').forEach(part => {
            part.addEventListener('mouseenter', () => {
                if (!bodyTip) return;
                bodyTip.textContent = part.dataset.tip;
                bodyFig.classList.add('showtip');
            });
            part.addEventListener('mouseleave', () => bodyFig.classList.remove('showtip'));
        });
    }

    const container = document.getElementById('avatar3d');
    const userId = container ? container.dataset.userid : null;
    if (userId) {
        window.init3DAvatar(userId);
    }
};

initProfile();

if (!window.__prfCardNavBound) {
    window.__prfCardNavBound = true;
    document.addEventListener('click', function(event) {
        const card = event.target.closest ? event.target.closest('#prfWearingRow .shp-card[data-id]') : null;
        if (card && !event.target.closest('button') && !event.target.closest('a')) {
            const id = card.getAttribute('data-id');
            if (id) {
                window.navigateWithHistory('/shop/item/' + encodeURIComponent(id));
            }
        }
    });
}

document.addEventListener('htmx:afterSettle', initProfile);
document.addEventListener('htmx:beforeTransition', () => {
    if (activeAvatarViewer) {
        activeAvatarViewer.destroy();
        activeAvatarViewer = null;
    }
});
window.addEventListener('resize', () => {
    if (typeof window.updateProfileTabIndicator === 'function') {
        window.updateProfileTabIndicator();
    }
    if (typeof window.updateWearingArrows === 'function') {
        window.updateWearingArrows();
    }
});
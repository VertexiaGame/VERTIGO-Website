import { AvatarViewer } from '/static/js/profile/profile.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';
import { OBJLoader } from 'three/addons/loaders/OBJLoader.js';
import { OBJExporter } from 'three/addons/exporters/OBJExporter.js';

const THREE = window.THREE;

const HIT_RADIUS = 28;
const HIT_RADIUS_TOUCH = 72;

const raycaster = new THREE.Raycaster();
const dragPlane = new THREE.Plane();

function defaultTransform() {
    return {
        position: { x: 0, y: 0, z: 0 },
        rotation: { x: 0, y: 0, z: 0 },
        scale: { x: 1, y: 1, z: 1 }
    };
}

function currentCreateCategory(fallback) {
    if (fallback) return fallback;
    const catTab = document.querySelector('.crtshoptabs .tabbtn.active');
    if (catTab && catTab.dataset.cat) return catTab.dataset.cat;
    return null;
}

function makeDefaultMaterial() {
    return new THREE.MeshStandardMaterial({ color: 0xffffff, roughness: 0.6, metalness: 0.05 });
}

function intersectRayPlane(ray, plane, target) {
    const denom = plane.normal.dot(ray.direction);
    if (Math.abs(denom) < 1e-8) return null;
    const t = -(plane.normal.dot(ray.origin) + plane.constant) / denom;
    if (t < 0) return null;
    target.copy(ray.direction).multiplyScalar(t).add(ray.origin);
    return target;
}

function loadModelFile(file) {
    const ext = (file.name.split('.').pop() || '').toLowerCase();
    const url = URL.createObjectURL(file);
    let promise;
    if (ext === 'glb') {
        promise = new GLTFLoader().loadAsync(url).then(gltf => gltf.scene);
    } else {
        promise = new OBJLoader().loadAsync(url);
    }
    return promise.finally(() => URL.revokeObjectURL(url));
}

async function extractModelTexture(model) {
    let texture = null;
    model.traverse(child => {
        if (texture || !child.isMesh) return;
        const mats = Array.isArray(child.material) ? child.material : [child.material];
        for (const m of mats) {
            if (m && m.map) {
                texture = m.map;
                return;
            }
        }
    });
    if (!texture || !texture.image) return null;
    const img = texture.image;
    const isDrawable = (typeof ImageBitmap !== 'undefined' && img instanceof ImageBitmap) ||
        (typeof HTMLImageElement !== 'undefined' && img instanceof HTMLImageElement) ||
        (typeof HTMLCanvasElement !== 'undefined' && img instanceof HTMLCanvasElement);
    if (!isDrawable) return null;
    const width = img.naturalWidth || img.width;
    const height = img.naturalHeight || img.height;
    if (!width || !height) return null;
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    canvas.getContext('2d').drawImage(img, 0, 0);
    return new Promise(resolve => canvas.toBlob(blob => resolve(blob), 'image/png'));
}

class ModelPreview {
    constructor(options = {}) {
        this.viewer = null;
        this.pivot = null;
        this.bodyBoxes = [];
        this.container = null;
        this.dragState = null;
        this.transform = null;
        this.modelFile = null;
        this.textureFile = null;
        this.cat = null;
        this.showError = options.showError || null;
        this.refreshTimer = null;
        this.refreshSeq = 0;
        this.previewAbort = null;

        this.onViewPointerDown = e => this.handlePointerDown(e);
        this.onWindowPointerMove = e => this.handlePointerMove(e);
        this.onWindowPointerUp = e => this.handlePointerUp(e);
    }

    getTransform() {
        if (!this.transform) {
            this.transform = defaultTransform();
        }
        const t = this.transform;
        if (typeof t.scale !== 'object' || t.scale === null) {
            const s = t.scale || 1;
            t.scale = { x: s, y: s, z: s };
        }
        return t;
    }

    setModel(file) {
        this.modelFile = file || null;
    }

    setTexture(file) {
        this.textureFile = file || null;
    }

    setTransform(transform) {
        if (transform === null || transform === undefined) {
            this.transform = null;
            return;
        }
        this.transform = transform;
        if (typeof transform.scale !== 'object' || transform.scale === null) {
            const s = transform.scale || 1;
            transform.scale = { x: s, y: s, z: s };
        }
    }

    setCategory(cat) {
        this.cat = cat;
    }

    get modal() {
        return document.getElementById('crt3dModal');
    }

    get viewEl() {
        return document.getElementById('crt3dView');
    }

    get canvas() {
        return this.viewer ? this.viewer.renderer.domElement : null;
    }

    syncInputs() {
        const t = this.getTransform();
        const set = (id, v) => {
            const el = document.getElementById(id);
            if (el) el.value = Math.round(v * 100) / 100;
        };
        set('crt3dPosX', t.position.x);
        set('crt3dPosY', t.position.y);
        set('crt3dPosZ', t.position.z);
        set('crt3dRotX', t.rotation.x);
        set('crt3dRotY', t.rotation.y);
        set('crt3dRotZ', t.rotation.z);
        set('crt3dScaleX', t.scale.x);
        set('crt3dScaleY', t.scale.y);
        set('crt3dScaleZ', t.scale.z);
    }

    readInputs() {
        const t = this.getTransform();
        const get = id => {
            const el = document.getElementById(id);
            const v = el ? parseFloat(el.value) : NaN;
            return isNaN(v) ? 0 : v;
        };
        t.position.x = get('crt3dPosX');
        t.position.y = get('crt3dPosY');
        t.position.z = get('crt3dPosZ');
        t.rotation.x = get('crt3dRotX');
        t.rotation.y = get('crt3dRotY');
        t.rotation.z = get('crt3dRotZ');
        t.scale.x = Math.max(0.05, get('crt3dScaleX'));
        t.scale.y = Math.max(0.05, get('crt3dScaleY'));
        t.scale.z = Math.max(0.05, get('crt3dScaleZ'));
        return t;
    }

    applyTransform() {
        if (!this.pivot) return;
        const t = this.getTransform();
        const baseScale = this.pivot.userData.baseScale || 1;

        this.pivot.scale.set(
            baseScale * t.scale.x,
            baseScale * t.scale.y,
            baseScale * t.scale.z
        );
        this.pivot.rotation.set(
            THREE.MathUtils.degToRad(t.rotation.x),
            THREE.MathUtils.degToRad(t.rotation.y),
            THREE.MathUtils.degToRad(t.rotation.z)
        );
        this.pivot.position.set(t.position.x, t.position.y, t.position.z);
    }

    scheduleRefresh() {
        if (this.refreshTimer) clearTimeout(this.refreshTimer);
        this.refreshTimer = setTimeout(() => {
            this.refreshTimer = null;
            this.refreshPreview();
        }, 350);
    }

    showPreviewSpinner(show) {
        const wrap = document.querySelector('#crtShopPreview .avtitmimg');
        if (!wrap) return;
        if (show) {
            if (wrap.querySelector('.musld')) return;
            const spinner = document.createElement('div');
            spinner.className = 'musld';
            spinner.style.position = 'absolute';
            spinner.style.inset = '0';
            spinner.style.zIndex = '10';
            spinner.style.pointerEvents = 'none';
            spinner.innerHTML = `
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
            wrap.appendChild(spinner);
        } else {
            const spinner = wrap.querySelector('.musld');
            if (spinner) spinner.remove();
        }
    }

    async refreshPreview(showError) {
        const img = document.getElementById('crtShopPreviewImg');
        if (!img) return;

        let cat = this.cat;
        if (!cat) {
            const catTab = document.querySelector('.crtshoptabs .tabbtn.active');
            if (catTab && catTab.dataset.cat) {
                cat = catTab.dataset.cat;
                this.cat = cat;
            }
        }
        if (!cat) return;

        const isModel = cat === 'hat' || cat === 'tool';
        if (!isModel && cat !== 'shirt' && cat !== 'tshirt' && cat !== 'pants' && cat !== 'face') return;
        const baseOnly = (isModel && !this.modelFile) || (!isModel && !this.textureFile);

        const seq = ++this.refreshSeq;
        if (this.previewAbort) {
            try { this.previewAbort.abort(); } catch {}
            this.previewAbort = null;
        }
        const aborter = new AbortController();
        this.previewAbort = aborter;
        this.showPreviewSpinner(true);

        let file = null;
        let textureFile = null;
        if (!baseOnly && isModel) {
            try {
                const exported = await this.exportModel();
                if (seq !== this.refreshSeq) return;
                if (!exported) return;
                file = exported.file;
                textureFile = exported.textureFile;
                if (seq !== this.refreshSeq) return;
            } catch {
                if (seq === this.refreshSeq) this.showPreviewSpinner(false);
                return;
            }
        } else if (!baseOnly) {
            file = this.textureFile;
        }

        const formData = new FormData();
        formData.append('category', cat);
        if (file) formData.append('file', file, file.name || 'model.obj');
        if (textureFile) {
            const texName = (textureFile.name && textureFile.name.indexOf('.') !== -1) ? textureFile.name : 'texture.png';
            const texToSend = textureFile.name ? textureFile : new File([textureFile], texName, { type: textureFile.type || 'image/png' });
            formData.append('texture', texToSend, texToSend.name || texName);
        }

        img.classList.remove('ld');

        try {
            const res = await fetch('/api/v1/create/preview', { method: 'POST', body: formData, signal: aborter.signal });
            if (!res.ok) {
                img.classList.add('ld');
                if ((res.status === 429 || res.status === 502 || res.status === 503 || res.status === 504) && seq === this.refreshSeq) {
                    if (this.refreshTimer) clearTimeout(this.refreshTimer);
                    this.refreshTimer = setTimeout(() => {
                        this.refreshTimer = null;
                        this.refreshPreview();
                    }, 1500);
                } else if (showError && this.showError && (res.status === 400 || res.status === 413 || res.status === 500) && seq === this.refreshSeq) {
                    try {
                        const msg = (await res.text()).trim().substring(0, 200);
                        if (msg) this.showError(msg);
                    } catch {}
                }
                return;
            }
            const blob = await res.blob();
            if (seq !== this.refreshSeq) return;
            const url = URL.createObjectURL(blob);
            img.onload = () => {
                img.classList.add('ld');
                URL.revokeObjectURL(url);
            };
            img.src = url;
        } catch (e) {
            if (e && e.name === 'AbortError') return;
            img.classList.add('ld');
            return;
        } finally {
            if (seq === this.refreshSeq) {
                this.showPreviewSpinner(false);
                if (this.previewAbort === aborter) this.previewAbort = null;
            }
        }
    }

    collisionDisabled() {
        const el = document.getElementById('crt3dNoCollide');
        return el ? el.checked : false;
    }

    updateColliders() {
        this.bodyBoxes = [];
        if (!this.viewer || !this.pivot) return;
        const group = this.viewer.avatarGroup;
        if (!group) return;
        group.updateMatrixWorld(true);
        const inv = new THREE.Matrix4().copy(group.matrixWorld).invert();
        for (const child of group.children) {
            if (!child || child === this.pivot) continue;
            const box = new THREE.Box3().setFromObject(child);
            if (box.isEmpty()) continue;
            box.applyMatrix4(inv);
            box.expandByScalar(-0.02);
            if (box.isEmpty()) continue;
            this.bodyBoxes.push(box);
        }
    }

    spawnPlacement() {
        if (!this.viewer || !this.pivot || !this.bodyBoxes.length) return;
        const group = this.viewer.avatarGroup;
        if (!group) return;
        group.updateMatrixWorld(true);
        const inv = new THREE.Matrix4().copy(group.matrixWorld).invert();
        const modelBox = new THREE.Box3().setFromObject(this.pivot);
        if (modelBox.isEmpty()) return;
        modelBox.applyMatrix4(inv);
        const size = modelBox.getSize(new THREE.Vector3());
        const base = modelBox.getCenter(new THREE.Vector3());
        const t = this.getTransform();
        if (this.cat === 'tool') {
            let arm = null;
            for (const bodyBox of this.bodyBoxes) {
                if (!arm || bodyBox.min.x < arm.min.x) arm = bodyBox;
            }
            if (!arm) return;
            const center = arm.getCenter(new THREE.Vector3());
            t.position.x = center.x - (base.x - this.pivot.position.x);
            t.position.y = center.y - (base.y - this.pivot.position.y);
            t.position.z = arm.max.z + size.z / 2 + 0.1 - (base.z - this.pivot.position.z);
        } else {
            let top = null;
            for (const bodyBox of this.bodyBoxes) {
                if (!top || bodyBox.max.y > top.max.y) top = bodyBox;
            }
            if (!top) return;
            const center = top.getCenter(new THREE.Vector3());
            t.position.x = center.x - (base.x - this.pivot.position.x);
            t.position.y = top.max.y + size.y / 2 + 0.15 - (base.y - this.pivot.position.y);
            t.position.z = center.z - (base.z - this.pivot.position.z);
        }
    }

    resolveCollision() {
        if (this.collisionDisabled()) return;
        if (!this.viewer || !this.pivot || !this.bodyBoxes.length) return;
        const group = this.viewer.avatarGroup;
        if (!group) return;
        group.updateMatrixWorld(true);
        const inv = new THREE.Matrix4().copy(group.matrixWorld).invert();
        const modelBox = new THREE.Box3().setFromObject(this.pivot);
        if (modelBox.isEmpty()) return;
        modelBox.applyMatrix4(inv);
        const t = this.getTransform();
        const minPush = 1e-6;
        for (let iter = 0; iter < 4; iter++) {
            let moved = false;
            for (const bodyBox of this.bodyBoxes) {
                if (!modelBox.intersectsBox(bodyBox)) continue;
                const bodyCenter = bodyBox.getCenter(new THREE.Vector3());
                if (modelBox.containsPoint(bodyCenter)) continue;
                const modelCenter = modelBox.getCenter(new THREE.Vector3());
                const ox = Math.min(modelBox.max.x - bodyBox.min.x, bodyBox.max.x - modelBox.min.x);
                const oy = Math.min(modelBox.max.y - bodyBox.min.y, bodyBox.max.y - modelBox.min.y);
                const oz = Math.min(modelBox.max.z - bodyBox.min.z, bodyBox.max.z - modelBox.min.z);
                if (ox < minPush && oy < minPush && oz < minPush) continue;
                const push = new THREE.Vector3();
                if (ox <= oy && ox <= oz) {
                    push.set(modelCenter.x < bodyCenter.x ? -ox : ox, 0, 0);
                } else if (oy <= ox && oy <= oz) {
                    push.set(0, modelCenter.y < bodyCenter.y ? -oy : oy, 0);
                } else {
                    push.set(0, 0, modelCenter.z < bodyCenter.z ? -oz : oz);
                }
                if (push.lengthSq() < minPush * minPush) continue;
                this.pivot.position.add(push);
                modelBox.translate(push);
                moved = true;
            }
            if (!moved) break;
        }
        t.position.x = this.pivot.position.x;
        t.position.y = this.pivot.position.y;
        t.position.z = this.pivot.position.z;
    }

    attachModel(model) {
        const textureLoader = new THREE.TextureLoader();
        let texture = null;
        if (this.textureFile) {
            const url = URL.createObjectURL(this.textureFile);
            texture = textureLoader.load(url);
            texture.colorSpace = THREE.SRGBColorSpace;
        }

        model.traverse(child => {
            if (child.isMesh) {
                child.castShadow = true;
                child.receiveShadow = true;
                if (!child.material || (Array.isArray(child.material) && child.material.length === 0)) {
                    child.material = makeDefaultMaterial();
                }
                if (texture) {
                    const mats = Array.isArray(child.material) ? child.material : [child.material];
                    mats.forEach(m => {
                        m.map = texture;
                        m.needsUpdate = true;
                    });
                }
            }
        });

        const pivot = new THREE.Group();
        pivot.userData.baseScale = 1;
        pivot.add(model);

        this.viewer.avatarGroup.add(pivot);
        this.pivot = pivot;
    }

    pointerToNDC(e) {
        const rect = this.canvas.getBoundingClientRect();
        return new THREE.Vector2(
            ((e.clientX - rect.left) / rect.width) * 2 - 1,
            -((e.clientY - rect.top) / rect.height) * 2 + 1
        );
    }

    hitsModel(e) {
        const ndc = this.pointerToNDC(e);
        raycaster.setFromCamera(ndc, this.viewer.camera);
        if (raycaster.intersectObject(this.pivot, true).length) return true;

        const center = this.pivot.getWorldPosition(new THREE.Vector3()).project(this.viewer.camera);
        const rect = this.canvas.getBoundingClientRect();
        const sx = (center.x * 0.5 + 0.5) * rect.width;
        const sy = (-center.y * 0.5 + 0.5) * rect.height;
        const px = e.clientX - rect.left;
        const py = e.clientY - rect.top;
        const radius = e.pointerType === 'touch' ? HIT_RADIUS_TOUCH : HIT_RADIUS;
        return Math.hypot(px - sx, py - sy) <= radius;
    }

    handlePointerDown(e) {
        if (!this.viewer || !this.pivot) return;
        if (this.dragState) {
            this.dragState = null;
            return;
        }
        if (this.viewer.activePointers && this.viewer.activePointers.size >= 1) {
            const activeCount = this.viewer.activePointers.size;
            if (activeCount >= 1 && e.isPrimary === false) return;
            if (activeCount >= 2) return;
        }
        if (!this.hitsModel(e)) return;
        e.stopImmediatePropagation();
        if (e.cancelable && e.pointerType === 'touch') {
            try { e.preventDefault(); } catch {}
        }

        const center = this.pivot.getWorldPosition(new THREE.Vector3());
        const normal = this.viewer.camera.getWorldDirection(new THREE.Vector3());
        dragPlane.setFromNormalAndCoplanarPoint(normal, center);

        const ndc = this.pointerToNDC(e);
        raycaster.setFromCamera(ndc, this.viewer.camera);
        const hit = new THREE.Vector3();
        if (!intersectRayPlane(raycaster.ray, dragPlane, hit)) return;

        const group = this.viewer.avatarGroup;
        group.updateWorldMatrix(true, true);
        const inv = new THREE.Matrix4().copy(group.matrixWorld).invert();
        const localHit = hit.clone().applyMatrix4(inv);

        this.dragState = {
            pointerId: e.pointerId,
            grabOffset: localHit.sub(this.pivot.position.clone())
        };
        try { this.container.setPointerCapture(e.pointerId); } catch {}
    }

    handlePointerMove(e) {
        const drag = this.dragState;
        if (!this.viewer || !this.pivot || !drag || drag.pointerId !== e.pointerId) return;
        if (this.viewer.activePointers && this.viewer.activePointers.size >= 2) {
            this.dragState = null;
            return;
        }
        if (e.cancelable && e.pointerType === 'touch') {
            try { e.preventDefault(); } catch {}
        }

        const ndc = this.pointerToNDC(e);
        raycaster.setFromCamera(ndc, this.viewer.camera);

        const center = this.pivot.getWorldPosition(new THREE.Vector3());
        const normal = this.viewer.camera.getWorldDirection(new THREE.Vector3());
        dragPlane.setFromNormalAndCoplanarPoint(normal, center);

        const hit = new THREE.Vector3();
        if (!intersectRayPlane(raycaster.ray, dragPlane, hit)) return;

        const group = this.viewer.avatarGroup;
        group.updateWorldMatrix(true, true);
        const inv = new THREE.Matrix4().copy(group.matrixWorld).invert();
        const localTarget = hit.clone().applyMatrix4(inv).sub(drag.grabOffset);

        const t = this.getTransform();
        t.position.x = localTarget.x;
        t.position.y = localTarget.y;
        t.position.z = localTarget.z;
        this.applyTransform();
        this.resolveCollision();
        this.syncInputs();
    }

    handlePointerUp(e) {
        const drag = this.dragState;
        if (drag && drag.pointerId === e.pointerId) {
            this.dragState = null;
            this.scheduleRefresh();
        } else if (this.viewer && this.viewer.activePointers && this.viewer.activePointers.size >= 2) {
            this.dragState = null;
        }
    }

    attachDragListeners() {
        this.container = this.viewEl;
        if (this.container && this.container.style) {
            this.container.style.touchAction = 'none';
        }
        this.container.addEventListener('pointerdown', this.onViewPointerDown, { capture: true, passive: false });
        this.container.addEventListener('contextmenu', this.onViewContextMenu || (this.onViewContextMenu = e => e.preventDefault()));
        window.addEventListener('pointermove', this.onWindowPointerMove, { passive: false });
        window.addEventListener('pointerup', this.onWindowPointerUp);
        window.addEventListener('pointercancel', this.onWindowPointerUp);
    }

    removeDragListeners() {
        if (this.container) {
            this.container.removeEventListener('pointerdown', this.onViewPointerDown, true);
            if (this.onViewContextMenu) this.container.removeEventListener('contextmenu', this.onViewContextMenu);
            this.container = null;
        }
        window.removeEventListener('pointermove', this.onWindowPointerMove);
        window.removeEventListener('pointerup', this.onWindowPointerUp);
        window.removeEventListener('pointercancel', this.onWindowPointerUp);
    }

    applyFromInputs() {
        this.transform = this.readInputs();
        this.applyTransform();
        this.resolveCollision();
        this.syncInputs();
        this.scheduleRefresh();
    }

    reset() {
        this.transform = null;
        this.syncInputs();
        this.applyTransform();
        this.scheduleRefresh();
    }

    getTransformData() {
        const t = this.getTransform();
        return {
            position: { x: t.position.x, y: t.position.y, z: t.position.z },
            rotation: { x: t.rotation.x, y: t.rotation.y, z: t.rotation.z },
            scale: { x: t.scale.x, y: t.scale.y, z: t.scale.z }
        };
    }

    async exportModel() {
        if (!this.modelFile) return null;
        const model = await loadModelFile(this.modelFile);
        const t = this.getTransform();
        const wrapper = new THREE.Group();
        wrapper.add(model);
        wrapper.position.set(t.position.x, t.position.y, t.position.z);
        wrapper.rotation.set(
            THREE.MathUtils.degToRad(t.rotation.x),
            THREE.MathUtils.degToRad(t.rotation.y),
            THREE.MathUtils.degToRad(t.rotation.z)
        );
        wrapper.scale.set(t.scale.x, t.scale.y, t.scale.z);
        wrapper.updateMatrixWorld(true);
        const file = new File([new OBJExporter().parse(wrapper)], 'model.obj', { type: 'text/plain' });
        const textureFile = this.textureFile || (await extractModelTexture(model));
        return { file, textureFile };
    }

    close() {
        this.dragState = null;
        this.removeDragListeners();
        if (this.viewer) {
            this.viewer.destroy();
            this.viewer = null;
            this.scheduleRefresh();
        }
        this.pivot = null;
        this.bodyBoxes = [];
        const modal = this.modal;
        if (modal) modal.classList.remove('active');
    }

    async open() {
        const file = this.modelFile;
        const modal = this.modal;
        const viewEl = this.viewEl;
        if (!file || !modal || !viewEl) return;

        this.close();
        modal.classList.add('active');
        viewEl.innerHTML = '';

        this.cat = currentCreateCategory(this.cat);
        this.viewer = new AvatarViewer(viewEl, 0, { dummy: true, toolPose: this.cat === 'tool' });
        this.viewer.autoRotate = false;
        const viewer = this.viewer;

        try {
            await viewer.load();
            if (this.viewer !== viewer) return;

            const model = await loadModelFile(file);
            if (!model) throw new Error('Failed to load model');
            if (this.viewer !== viewer || !modal.classList.contains('active')) return;

            const fresh = !this.transform;
            this.attachModel(model);
            this.updateColliders();
            if (fresh) this.spawnPlacement();
            this.syncInputs();
            this.applyTransform();
            this.attachDragListeners();
        } catch (e) {
            if (this.viewer === viewer) {
                this.close();
                if (this.showError) {
                    this.showError('Could not load the 3D model. Make sure it is a valid GLB or OBJ file.');
                }
            }
        }
    }
}

const preview = new ModelPreview({ showError: window.createShowError || null });

const initialCatTab = document.querySelector('.crtshoptabs .tabbtn.active');
if (initialCatTab && initialCatTab.dataset.cat) preview.setCategory(initialCatTab.dataset.cat);

window.openCreate3DPreview = function() { preview.open(); };
window.closeCreate3DPreview = function() { preview.close(); };
window.applyCreateTransform = function() { preview.applyFromInputs(); };
window.resetCreateTransform = function() { preview.reset(); };
window.setCreateModelFile = function(file) { preview.setModel(file); };
window.setCreateTextureFile = function(file) { preview.setTexture(file); };
window.setCreateCategory = function(cat) { preview.setCategory(cat); };
window.getCreateModelFile = function() { return preview.modelFile; };
window.getCreateTransformedModel = async function() {
    try {
        const exported = await preview.exportModel();
        return exported ? exported.file : null;
    } catch (e) {
        return null;
    }
};
window.refreshCreatePreview = function(showError) { preview.refreshPreview(showError); };
window.getCreateModelTransformData = function() { return preview.getTransformData(); };

if (window.pendingCreateModelFile && !preview.modelFile) {
    preview.setModel(window.pendingCreateModelFile);
    window.pendingCreateModelFile = null;
}
if (window.pendingCreateTextureFile && !preview.textureFile) {
    preview.setTexture(window.pendingCreateTextureFile);
    window.pendingCreateTextureFile = null;
}
if (!preview.modelFile) {
    const modelInput = document.getElementById('crtShopModelFile');
    const modelPicked = modelInput && modelInput.files && modelInput.files[0];
    if (modelPicked) preview.setModel(modelPicked);
}
if (!preview.textureFile) {
    const texInput = document.getElementById('crtShopFile');
    const texPicked = texInput && texInput.files && texInput.files[0];
    if (texPicked) preview.setTexture(texPicked);
}

document.addEventListener('htmx:beforeTransition', function() {
    preview.close();
});

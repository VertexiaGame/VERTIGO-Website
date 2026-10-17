let adminUptimeTimer = null;
let adminPollTimer = null;
let currentAdminUserPage = 1;
let currentAdminLogsPage = 1;
let currentAdminAssetPage = 1;
let currentAdminShopItemPage = 1;
let currentAdminBannerPage = 1;
const activeAdmin3DViewers = new Map();

const adminAssetTypeNames = {
    image: 'Image',
    mesh: 'Mesh',
    sound: 'Sound'
};

const adminShopItemTypeNames = {
    hat: 'Hat',
    faces: 'Face',
    shirts: 'Shirt',
    tshirts: 'T-Shirt',
    pants: 'Pants',
    gear: 'Gear'
};

if (typeof window.escapeHTML !== 'function') {
    window.escapeHTML = function(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    };
}

class AdminMeshViewer {
    constructor(container, assetId, filePath) {
        this.container = container;
        this.assetId = assetId;
        this.filePath = filePath || '';
        this.isDestroyed = false;
        this.animId = null;
        this.isDragging = false;
        this.pointer = { x: 0, y: 0 };
        this.spherical = {
            radius: 3.6,
            theta: 0.5,
            phi: 0.35
        };
        this.targetRadius = 3.6;
        this.minRadius = 1.2;
        this.maxRadius = 8.0;
        this.target = null;

        this.init();
    }

    async init() {
        this.container.innerHTML = '';

        const THREE = window.THREE || (await import('three'));
        const { GLTFLoader } = await import('three/addons/loaders/GLTFLoader.js');
        const { OBJLoader } = await import('three/addons/loaders/OBJLoader.js');

        if (this.isDestroyed) return;

        this.THREE = THREE;
        this.GLTFLoader = GLTFLoader;
        this.OBJLoader = OBJLoader;

        this.target = new THREE.Vector3(0, 0, 0);

        const width = this.container.clientWidth || 300;
        const height = this.container.clientHeight || 300;

        this.scene = new THREE.Scene();
        this.camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 100);
        this.updateCamera();

        this.renderer = new THREE.WebGLRenderer({ alpha: true, antialias: true });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        if (THREE.NeutralToneMapping) {
            this.renderer.toneMapping = THREE.NeutralToneMapping;
        }
        this.renderer.toneMappingExposure = 1.0;
        if (this.renderer.domElement && this.renderer.domElement.style) {
            this.renderer.domElement.style.touchAction = 'none';
            this.renderer.domElement.style.display = 'block';
        }
        if (this.container && this.container.style) {
            this.container.style.touchAction = 'none';
        }
        this.container.appendChild(this.renderer.domElement);

        const hemiLight = new THREE.HemisphereLight(0xffffff, 0x888888, 0.85);
        this.scene.add(hemiLight);

        const keyLight = new THREE.DirectionalLight(0xffffff, 1.0);
        keyLight.position.set(4, 6, 5);
        this.scene.add(keyLight);

        const fillLight = new THREE.DirectionalLight(0xffffff, 0.35);
        fillLight.position.set(-4, 3, -3);
        this.scene.add(fillLight);

        this.wrapper = new THREE.Group();
        this.scene.add(this.wrapper);

        this.showLoading();
        this.bindEvents();
        this.initResize();

        try {
            await this.loadModel();
            this.hideLoading();
            if (!this.isDestroyed) {
                this.animate();
            }
        } catch (err) {
            this.hideLoading();
            this.showError();
        }
    }

    showLoading() {
        this.spinner = document.createElement('div');
        this.spinner.className = 'musld';
        this.spinner.style.position = 'absolute';
        this.spinner.style.top = '50%';
        this.spinner.style.left = '50%';
        this.spinner.style.transform = 'translate(-50%, -50%)';
        this.spinner.style.zIndex = '10';
        this.spinner.style.pointerEvents = 'none';
        this.spinner.innerHTML = `
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
        this.container.appendChild(this.spinner);
    }

    hideLoading() {
        if (this.spinner && this.spinner.parentNode) {
            this.spinner.parentNode.removeChild(this.spinner);
            this.spinner = null;
        }
    }

    showError() {
        const errEl = document.createElement('div');
        errEl.className = 'adm-3d-err';
        errEl.textContent = 'Could not render 3D model';
        this.container.appendChild(errEl);
    }

    async loadModel() {
        const res = await fetch(`/api/v1/assets/${this.assetId}/file`);
        if (!res.ok) throw new Error('Failed to fetch asset');

        const arrayBuffer = await res.arrayBuffer();
        if (this.isDestroyed) return;

        const isGLB = this.filePath.toLowerCase().endsWith('.glb') ||
            (arrayBuffer.byteLength >= 4 && new Uint32Array(arrayBuffer.slice(0, 4))[0] === 0x46546C67);

        let model;
        if (isGLB) {
            const gltfLoader = new this.GLTFLoader();
            const gltf = await gltfLoader.parseAsync(arrayBuffer, '');
            model = gltf.scene;
        } else {
            const text = new TextDecoder().decode(arrayBuffer);
            const objLoader = new this.OBJLoader();
            model = objLoader.parse(text);
        }

        const THREE = this.THREE;
        model.traverse(child => {
            if (child.isMesh) {
                child.castShadow = true;
                child.receiveShadow = true;
                if (!child.material || (Array.isArray(child.material) && child.material.length === 0)) {
                    child.material = new THREE.MeshStandardMaterial({
                        color: 0xd6d6d6,
                        roughness: 0.6,
                        metalness: 0.1
                    });
                }
                if (child.geometry && !child.geometry.attributes.normal) {
                    child.geometry.computeVertexNormals();
                }
            }
        });

        model.updateMatrixWorld(true);
        const box = new THREE.Box3().setFromObject(model);
        const center = box.getCenter(new THREE.Vector3());
        const size = box.getSize(new THREE.Vector3());
        const maxDim = Math.max(size.x, size.y, size.z) || 1;
        const scale = 2.4 / maxDim;

        model.position.sub(center);

        this.wrapper.scale.setScalar(scale);
        this.wrapper.add(model);
    }

    updateCamera() {
        if (!this.target) return;
        this.spherical.radius += (this.targetRadius - this.spherical.radius) * 0.1;
        const x = this.target.x + this.spherical.radius * Math.sin(this.spherical.theta) * Math.cos(this.spherical.phi);
        const y = this.target.y + this.spherical.radius * Math.sin(this.spherical.phi);
        const z = this.target.z + this.spherical.radius * Math.cos(this.spherical.theta) * Math.cos(this.spherical.phi);
        this.camera.position.set(x, y, z);
        this.camera.lookAt(this.target);
    }

    animate() {
        if (this.isDestroyed) return;
        this.animId = requestAnimationFrame(() => this.animate());

        if (!this.isDragging) {
            this.spherical.theta += 0.006;
        }

        this.updateCamera();
        this.renderer.render(this.scene, this.camera);
    }

    bindEvents() {
        this.activePointers = new Map();
        this.initialPinchDistance = null;

        this.boundPointerDown = e => {
            if (e.cancelable && e.pointerType === 'touch') {
                try { e.preventDefault(); } catch (_) {}
            }
            try { this.container.setPointerCapture(e.pointerId); } catch (_) {}
            this.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
            if (this.activePointers.size === 1) {
                this.isDragging = true;
                this.pointer = { x: e.clientX, y: e.clientY };
            } else if (this.activePointers.size === 2) {
                this.isDragging = false;
                const pts = Array.from(this.activePointers.values());
                this.initialPinchDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
            }
        };

        this.boundPointerMove = e => {
            if (!this.activePointers.has(e.pointerId)) return;
            if (e.cancelable && e.pointerType === 'touch') {
                try { e.preventDefault(); } catch (_) {}
            }
            this.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
            if (this.activePointers.size === 1 && this.isDragging) {
                const dx = e.clientX - this.pointer.x;
                const dy = e.clientY - this.pointer.y;
                this.pointer = { x: e.clientX, y: e.clientY };

                this.spherical.theta -= dx * 0.008;
                this.spherical.phi += dy * 0.008;
                this.spherical.phi = Math.max(-1.45, Math.min(1.45, this.spherical.phi));
            } else if (this.activePointers.size === 2 && this.initialPinchDistance !== null) {
                const pts = Array.from(this.activePointers.values());
                const currentDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
                if (currentDistance > 0) {
                    const deltaDistance = this.initialPinchDistance - currentDistance;
                    this.targetRadius = Math.max(this.minRadius, Math.min(this.maxRadius, this.targetRadius + deltaDistance * 0.01));
                    this.initialPinchDistance = currentDistance;
                }
            }
        };

        this.boundPointerUp = e => {
            this.activePointers.delete(e.pointerId);
            if (this.activePointers.size < 2) {
                this.initialPinchDistance = null;
            }
            if (this.activePointers.size === 1) {
                this.isDragging = true;
                const p = Array.from(this.activePointers.values())[0];
                this.pointer = { x: p.x, y: p.y };
            } else if (this.activePointers.size === 0) {
                this.isDragging = false;
            }
            try { this.container.releasePointerCapture(e.pointerId); } catch (_) {}
        };

        this.boundWheel = e => {
            e.preventDefault();
            this.targetRadius = Math.max(this.minRadius, Math.min(this.maxRadius, this.targetRadius + e.deltaY * 0.004));
        };

        this.boundContextMenu = e => e.preventDefault();

        this.container.addEventListener('pointerdown', this.boundPointerDown, { passive: false });
        this.container.addEventListener('pointermove', this.boundPointerMove, { passive: false });
        this.container.addEventListener('pointerup', this.boundPointerUp);
        this.container.addEventListener('pointercancel', this.boundPointerUp);
        this.container.addEventListener('contextmenu', this.boundContextMenu);
        this.container.addEventListener('wheel', this.boundWheel, { passive: false });
    }

    initResize() {
        if (window.ResizeObserver) {
            this.ro = new ResizeObserver(entries => {
                for (const entry of entries) {
                    const w = entry.contentRect.width;
                    const h = entry.contentRect.height;
                    if (w > 0 && h > 0 && this.renderer && this.camera) {
                        this.camera.aspect = w / h;
                        this.camera.updateProjectionMatrix();
                        this.renderer.setSize(w, h);
                    }
                }
            });
            this.ro.observe(this.container);
        }
    }

    destroy() {
        this.isDestroyed = true;
        if (this.animId) cancelAnimationFrame(this.animId);
        if (this.ro) this.ro.disconnect();

        this.container.removeEventListener('pointerdown', this.boundPointerDown);
        this.container.removeEventListener('pointermove', this.boundPointerMove);
        this.container.removeEventListener('pointerup', this.boundPointerUp);
        this.container.removeEventListener('pointercancel', this.boundPointerUp);
        this.container.removeEventListener('contextmenu', this.boundContextMenu);
        this.container.removeEventListener('wheel', this.boundWheel);

        if (this.scene) {
            this.scene.traverse(obj => {
                if (obj.geometry) obj.geometry.dispose();
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

        this.container.innerHTML = '';
    }
}

window.toggleAdminMesh3D = function(assetId, buttonEl) {
    const card = buttonEl.closest('.adm-card');
    if (!card) return;

    const renderCrd = card.querySelector('.adm-rendcrd');
    if (!renderCrd) return;

    const img = renderCrd.querySelector('.adm-rendimg');
    let wrap3d = renderCrd.querySelector('.adm-3d-canvas-wrap');

    if (activeAdmin3DViewers.has(assetId)) {
        const viewer = activeAdmin3DViewers.get(assetId);
        viewer.destroy();
        activeAdmin3DViewers.delete(assetId);
        if (wrap3d) wrap3d.remove();
        if (img) img.style.display = '';
        buttonEl.classList.remove('active');
        buttonEl.querySelector('span').textContent = '3D';
        return;
    }

    if (img) img.style.display = 'none';

    wrap3d = document.createElement('div');
    wrap3d.className = 'adm-3d-canvas-wrap';
    renderCrd.appendChild(wrap3d);

    buttonEl.classList.add('active');
    buttonEl.querySelector('span').textContent = '2D';

    const filePath = buttonEl.dataset.filepath || '';
    const viewer = new AdminMeshViewer(wrap3d, assetId, filePath);
    activeAdmin3DViewers.set(assetId, viewer);
};

window.destroyAllAdmin3DViewers = function() {
    activeAdmin3DViewers.forEach(viewer => viewer.destroy());
    activeAdmin3DViewers.clear();
};

window.updateAdminTabIndicator = function() {
    const indicator = document.querySelector('.tabcnt:not(.adm-cat-tabs) .tabind');
    const activeTab = document.querySelector('.tabcnt:not(.adm-cat-tabs) .tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.updateAssetTabIndicator = function() {
    const indicator = document.querySelector('.adm-cat-tabs .tabind');
    const activeTab = document.querySelector('.adm-cat-tabs .tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.updateAdminViewportHeight = function() {
    const activeContent = document.querySelector('.settabcntnt.active');
    const viewport = document.querySelector('.settabvwp');
    if (activeContent && viewport) {
        viewport.style.height = activeContent.offsetHeight + 'px';
        if (window.adminHeightTimeout) {
            clearTimeout(window.adminHeightTimeout);
        }
        window.adminHeightTimeout = setTimeout(() => {
            viewport.style.height = '';
        }, 350);
    }
};

window.updateAdminUrl = function(tabName) {
    try {
        const url = new URL(window.location.href);
        url.searchParams.set('tab', tabName);
        window.history.replaceState({}, document.title, url.pathname + '?' + url.searchParams.toString());
    } catch (e) {}
};

window.switchAdminTab = function(tabName, event) {
    const tabs = Array.from(document.querySelectorAll('.tabcnt:not(.adm-cat-tabs) .tabbtn'));
    const clickedTab = event ? event.currentTarget : null;
    if (!clickedTab || clickedTab.classList.contains('active')) return;

    const currentContent = document.querySelector('.settabcntnt.active');
    const targetContent = document.getElementById('tab-' + tabName);
    if (!targetContent) return;

    window.destroyAllAdmin3DViewers();

    tabs.forEach(tab => tab.classList.remove('active'));
    clickedTab.classList.add('active');

    window.updateAdminTabIndicator();
    window.updateAdminUrl(tabName);

    if (tabName === 'servers' && window.fetchAdminGServers) {
        window.fetchAdminGServers();
    }

    const viewport = document.querySelector('.settabvwp');
    const currentHeight = currentContent ? currentContent.offsetHeight : 0;

    if (viewport) {
        viewport.style.height = currentHeight + 'px';
        viewport.offsetHeight;

        if (currentContent) {
            currentContent.classList.remove('active');
        }
        targetContent.classList.add('active');

        window.updateAdminViewportHeight();
    }
};

const formatUptimeJS = (seconds) => {
    if (seconds < 0) seconds = 0;
    const days = Math.floor(seconds / 86400);
    seconds %= 86400;
    const hours = Math.floor(seconds / 3600);
    seconds %= 3600;
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;

    if (days > 0) return `${days}d ${hours}h ${mins}m ${secs}s`;
    if (hours > 0) return `${hours}h ${mins}m ${secs}s`;
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
};

const updateLiveUptime = () => {
    const el = document.getElementById('admUptime');
    if (!el) return;
    const startMs = parseInt(el.dataset.startTimeMs, 10);
    if (!startMs) return;

    const diffSec = Math.floor((Date.now() - startMs) / 1000);
    el.textContent = formatUptimeJS(diffSec);
};

const pollAdminStatus = () => {
    fetch('/api/v1/admin/status')
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            const cpuEl = document.getElementById('admCpuUsage');
            if (cpuEl) cpuEl.textContent = data.cpu_usage;

            const allocEl = document.getElementById('admMemAlloc');
            if (allocEl) allocEl.textContent = data.mem_alloc;

            const sysEl = document.getElementById('admMemSys');
            if (sysEl) sysEl.textContent = data.mem_sys;

            const hostEl = document.getElementById('admHostMem');
            if (hostEl) hostEl.textContent = `${data.sys_mem_used} / ${data.sys_mem_total} (${data.sys_mem_pct})`;

            const goEl = document.getElementById('admGoroutines');
            if (goEl) goEl.textContent = data.goroutines;

            const gcEl = document.getElementById('admGcCycles');
            if (gcEl) gcEl.textContent = data.gc_cycles;
        })
        .catch(() => {});
    const serversTab = document.getElementById('tab-servers');
    if (serversTab && serversTab.classList.contains('active') && window.fetchAdminGServers) {
        window.fetchAdminGServers();
    }
};

window.fetchAdminUsers = function(page = 1) {
    currentAdminUserPage = page;
    const searchInput = document.getElementById('admUserSearch');
    const roleSelect = document.getElementById('admRoleFilter');
    const q = searchInput ? searchInput.value.trim() : '';
    const role = roleSelect ? roleSelect.value : '-1';

    const grid = document.getElementById('admUsersGrid');
    if (!grid) return;

    fetch(`/api/v1/admin/users?q=${encodeURIComponent(q)}&role=${role}&page=${page}&limit=15`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminUserTable(data);
        })
        .catch(() => {});
};

window.renderAdminUserTable = function(data) {
    const grid = document.getElementById('admUsersGrid');
    const pgnTxt = document.getElementById('admPgnTxt');
    const prevBtn = document.getElementById('admPrevBtn');
    const nextBtn = document.getElementById('admNextBtn');

    if (!grid) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.users || data.users.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No users found</div>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.users.forEach(u => {
        const roleClass = u.role_name === 'head manager' ? 'head-manager' : (u.role_name === 'vertexia-team' ? 'vertexia-team' : u.role_name);
        const safeUsername = window.escapeHTML(u.username);
        const safeDisplayName = window.escapeHTML(u.display_name);

        html += `
            <div class="frncrd">
                <div class="frntop">
                    <a href="/user/${u.id}" hx-get="/user/${u.id}" hx-target="body" hx-push-url="true" class="frnavt">
                        <img src="/api/v1/avatar/headshot/${u.id}.png" onerror="this.src='/static/useful/temp/pfp.png';" alt="${safeUsername}">
                    </a>
                    <div class="frndtl">
                        <a href="/user/${u.id}" hx-get="/user/${u.id}" hx-target="body" hx-push-url="true" class="frndn">${safeDisplayName}</a>
                        <span class="frnun">@${safeUsername} • #${u.id}</span>
                    </div>
                </div>
                <div class="adm-crdmeta">
                    <span class="adm-badge adm-badge-${roleClass}">${window.escapeHTML(u.role_display)}</span>
                    <span>Joined ${u.creation_date}</span>
                </div>
                <div class="frnact">
                    <button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.viewAdminUser(${u.id})">
                        <span>View User</span>
                    </button>
                </div>
            </div>
        `;
    });

    grid.innerHTML = html;

    if (pgnTxt) {
        pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    }

    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.viewAdminUser = function(userId) {
    if (window.navigateWithHistory) {
        window.navigateWithHistory(`/admin/users/${userId}`);
    } else if (window.htmx) {
        htmx.ajax('GET', `/admin/users/${userId}`, { target: 'body' });
    } else {
        window.location.href = `/admin/users/${userId}`;
    }
};

window.fetchAdminLogs = function(page = 1) {
    currentAdminLogsPage = page;
    const body = document.getElementById('admLogsBody');
    if (!body) return;

    fetch(`/api/v1/admin/logs?page=${page}&limit=15`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminLogsTable(data);
        })
        .catch(() => {});
};

window.renderAdminLogsTable = function(data) {
    const body = document.getElementById('admLogsBody');
    const pgnTxt = document.getElementById('admLogsPgnTxt');
    const prevBtn = document.getElementById('admLogsPrevBtn');
    const nextBtn = document.getElementById('admLogsNextBtn');

    if (!body) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.logs || data.logs.length === 0) {
        body.innerHTML = '<tr><td colspan="7" class="adm-tblemt">No admin actions logged</td></tr>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.logs.forEach(log => {
        const statusHtml = log.status === 'active'
            ? `<span class="adm-stat"><span class="adm-ind adm-ind-offline"></span>${window.escapeHTML(log.status_label)}</span>`
            : `<span class="adm-stat"><span class="adm-ind adm-ind-online"></span>${window.escapeHTML(log.status_label)}</span>`;

        html += `
            <tr>
                <td>#${log.id}</td>
                <td>${window.escapeHTML(log.admin_name)}</td>
                <td><strong class="adm-act">${window.escapeHTML(log.action_label)}</strong></td>
                <td>@${window.escapeHTML(log.target_name)} #${log.target_id}</td>
                <td>${window.escapeHTML(log.reason)}</td>
                <td>${statusHtml}</td>
                <td>${window.escapeHTML(log.date)}</td>
            </tr>
        `;
    });

    body.innerHTML = html;

    if (pgnTxt) {
        pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    }

    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.filterAdminAssetCategory = function(category, event) {
    const filterInput = document.getElementById('admAssetFilter');
    if (filterInput) filterInput.value = category;

    const clickedBtn = event ? event.currentTarget : null;
    if (clickedBtn) {
        document.querySelectorAll('.adm-cat-tabs .tabbtn').forEach(btn => {
            btn.classList.remove('active');
        });
        clickedBtn.classList.add('active');
        window.updateAssetTabIndicator();
    }

    window.fetchAdminAssets(1);
};

window.fetchAdminAssets = function(page = 1) {
    window.destroyAllAdmin3DViewers();
    currentAdminAssetPage = page;
    const filter = document.getElementById('admAssetFilter');
    const category = filter ? filter.value : '';

    const grid = document.getElementById('admAssetsGrid');
    if (!grid) return;

    fetch(`/api/v1/admin/assets?category=${encodeURIComponent(category)}&page=${page}&limit=12`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminAssets(data);
        })
        .catch(() => {});
};

window.renderAdminAssets = function(data) {
    window.destroyAllAdmin3DViewers();
    const grid = document.getElementById('admAssetsGrid');
    const pgnTxt = document.getElementById('admAssetsPgnTxt');
    const prevBtn = document.getElementById('admAssetsPrevBtn');
    const nextBtn = document.getElementById('admAssetsNextBtn');

    if (!grid) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.assets || data.assets.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No assets awaiting review</div>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.assets.forEach(asset => {
        const safeName = window.escapeHTML(asset.name);
        const safeOwner = window.escapeHTML(asset.owner_name);
        const safeDesc = window.escapeHTML(asset.description);
        const safeTypeName = window.escapeHTML(adminAssetTypeNames[asset.type] || asset.type);
        const safeFilePath = window.escapeHTML(asset.file_path || '');
        const typeIcon = asset.type === 'image' ? 'fa-image' : (asset.type === 'mesh' ? 'fa-cube' : 'fa-music');

        let dateStr = '';
        if (asset.created_at) {
            const created = new Date(asset.created_at);
            dateStr = created.toLocaleString('en-US', {
                month: 'short', day: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit', hour12: true
            });
        }

        let preview = '';
        if (asset.type === 'image') {
            preview = `<img src="/api/v1/assets/${asset.id}/file" alt="${safeName}" class="adm-rendimg" loading="lazy" onerror="this.src='/static/useful/temp/pfp.png';">`;
        } else if (asset.type === 'sound') {
            preview = `
                <div class="adm-audio-wrap">
                    <i class="fa-solid fa-music adm-audio-ico"></i>
                    <audio controls src="/api/v1/assets/${asset.id}/file" class="adm-audio-player"></audio>
                </div>
            `;
        } else {
            preview = `
                <img src="/api/v1/assets/${asset.id}/render" alt="${safeName}" class="adm-rendimg" loading="lazy" onerror="this.src='/static/useful/temp/pfp.png';">
                <button type="button" class="adm-3d-toggle-btn" data-filepath="${safeFilePath}" onclick="window.toggleAdminMesh3D(${asset.id}, this)" title="3D Preview">
                    <i class="fa-solid fa-cube"></i>
                    <span>3D</span>
                </button>
            `;
        }

        html += `
            <div class="adm-card" data-id="${asset.id}">
                <div class="adm-cardhdr">
                    <h3 class="adm-cardttl"><i class="fa-solid ${typeIcon}"></i><span>${safeName}</span></h3>
                    <span class="adm-badge adm-badge-${asset.type}">${safeTypeName}</span>
                </div>
                <div class="adm-rendcrd">
                    ${preview}
                </div>
                <div class="adm-rowlst">
                    <div class="adm-row"><span class="adm-lbl">Owner</span><span class="adm-val"><a href="/user/${asset.uid}" hx-get="/user/${asset.uid}" hx-target="body" hx-push-url="true" class="adm-owner-link">@${safeOwner}</a></span></div>
                    <div class="adm-row"><span class="adm-lbl">Submitted</span><span class="adm-val">${dateStr}</span></div>
                    ${safeDesc ? `<div class="adm-row"><span class="adm-lbl">Description</span><span class="adm-val">${safeDesc}</span></div>` : ''}
                </div>
                <div class="adm-modbtns">
                    <button type="button" class="primary hpysuc hpyinl hpysm" onclick="window.reviewAdminAsset(${asset.id}, 'approve')">
                        <span>Approve</span>
                    </button>
                    <button type="button" class="primary hpydng hpyinl hpysm" onclick="window.promptRejectAsset(${asset.id}, '${safeName}')">
                        <span>Reject</span>
                    </button>
                    <a href="/api/v1/assets/${asset.id}/file" target="_blank" rel="noopener" class="primary hpyinf hpyinl hpysm" style="text-decoration: none;">
                        <span>Download</span>
                    </a>
                </div>
            </div>
        `;
    });

    grid.innerHTML = html;

    if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.promptRejectAsset = function(id, name) {
    if (window.showModal) {
        const safeName = window.escapeHTML(name || 'this asset');
        window.showModal({
            title: 'Reject Asset',
            subtitle: `Asset #${id} (${safeName})`,
            body: `
                <form action="javascript:void(0);" onsubmit="window.confirmRejectAsset(${id}, event)" style="display: flex; flex-direction: column; gap: 12px;">
                    <label class="adm-lbl" style="color: #3D3D3D !important; font-weight: 500;">Rejection Reason (optional):</label>
                    <div class="setinbox">
                        <input type="text" id="admRejectModalNote" class="setin" placeholder="Reason for rejection" maxlength="500" autocomplete="off">
                    </div>
                    <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px;">
                        <button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()">
                            <span>Cancel</span>
                        </button>
                        <button type="submit" class="primary hpydng hpyinl hpysm">
                            <span>Reject</span>
                        </button>
                    </div>
                </form>
            `
        });
        setTimeout(() => {
            const input = document.getElementById('admRejectModalNote');
            if (input) input.focus();
        }, 50);
    } else {
        window.reviewAdminAsset(id, 'reject', '');
    }
};

window.confirmRejectAsset = function(id, event) {
    if (event) event.preventDefault();
    const input = document.getElementById('admRejectModalNote');
    const note = input ? input.value.trim() : '';
    if (window.closeModal) window.closeModal();
    window.reviewAdminAsset(id, 'reject', note);
};

window.updateShopItemTabIndicator = function() {
    const indicator = document.querySelector('.adm-shopcat-tabs .tabind');
    const activeTab = document.querySelector('.adm-shopcat-tabs .tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.filterAdminShopItemCategory = function(category, event) {
    const filterInput = document.getElementById('admShopItemFilter');
    if (filterInput) filterInput.value = category;

    const clickedBtn = event ? event.currentTarget : null;
    if (clickedBtn) {
        document.querySelectorAll('.adm-shopcat-tabs .tabbtn').forEach(btn => {
            btn.classList.remove('active');
        });
        clickedBtn.classList.add('active');
        window.updateShopItemTabIndicator();
    }

    window.fetchAdminShopItems(1);
};

window.fetchAdminShopItems = function(page = 1) {
    currentAdminShopItemPage = page;
    const filter = document.getElementById('admShopItemFilter');
    const category = filter ? filter.value : '';

    const grid = document.getElementById('admShopItemsGrid');
    if (!grid) return;

    fetch(`/api/v1/admin/shop?category=${encodeURIComponent(category)}&page=${page}&limit=12`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminShopItems(data);
        })
        .catch(() => {});
};

window.adminShopItemPrice = function(item) {
    const bucks = Number(item.bucks) || 0;
    const bits = Number(item.bits) || 0;
    if (bucks <= 0 && bits <= 0) return 'Free';
    const parts = [];
    if (bucks > 0) parts.push(`${bucks} Vertices`);
    if (bits > 0) parts.push(`${bits} Tickets`);
    return parts.join(' + ');
};

window.renderAdminShopItems = function(data) {
    const grid = document.getElementById('admShopItemsGrid');
    const pgnTxt = document.getElementById('admShopItemsPgnTxt');
    const prevBtn = document.getElementById('admShopItemsPrevBtn');
    const nextBtn = document.getElementById('admShopItemsNextBtn');

    if (!grid) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.items || data.items.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No shop items awaiting review</div>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.items.forEach(item => {
        const safeName = window.escapeHTML(item.name);
        const safeOwner = window.escapeHTML(item.creator_name);
        const safeDesc = window.escapeHTML(item.description);
        const safeTypeName = window.escapeHTML(adminShopItemTypeNames[item.type] || item.type);
        const typeIcon = item.type === 'faces' ? 'fa-face-smile' : ((item.type === 'shirts' || item.type === 'tshirts' || item.type === 'pants') ? 'fa-shirt' : 'fa-cube');

        let dateStr = '';
        if (item.createdat) {
            const created = new Date(item.createdat);
            dateStr = created.toLocaleString('en-US', {
                month: 'short', day: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit', hour12: true
            });
        }

        html += `
            <div class="adm-card" data-id="${item.id}">
                <div class="adm-cardhdr">
                    <h3 class="adm-cardttl"><i class="fa-solid ${typeIcon}"></i><span>${safeName}</span></h3>
                    <span class="adm-badge adm-badge-${item.type}">${safeTypeName}</span>
                </div>
                <div class="adm-rendcrd">
                    <img src="/avatar/shop/${item.type}/${item.id}" alt="${safeName}" class="adm-rendimg" loading="lazy" onerror="this.src='/static/useful/temp/pfp.png';">
                </div>
                <div class="adm-rowlst">
                    <div class="adm-row"><span class="adm-lbl">Owner</span><span class="adm-val"><a href="/user/${item.cid}" hx-get="/user/${item.cid}" hx-target="body" hx-push-url="true" class="adm-owner-link">@${safeOwner}</a></span></div>
                    <div class="adm-row"><span class="adm-lbl">Submitted</span><span class="adm-val">${dateStr}</span></div>
                    <div class="adm-row"><span class="adm-lbl">Price</span><span class="adm-val">${window.escapeHTML(window.adminShopItemPrice(item))}</span></div>
                    ${safeDesc ? `<div class="adm-row"><span class="adm-lbl">Description</span><span class="adm-val">${safeDesc}</span></div>` : ''}
                </div>
                <div class="adm-modbtns">
                    <button type="button" class="primary hpysuc hpyinl hpysm" onclick="window.reviewAdminShopItem(${item.id}, 'approve')">
                        <span>Approve</span>
                    </button>
                    <button type="button" class="primary hpydng hpyinl hpysm" onclick="window.promptRejectShopItem(${item.id}, '${safeName}')">
                        <span>Reject</span>
                    </button>
                    <a href="/shop/item/${item.id}" target="_blank" rel="noopener" class="primary hpyinf hpyinl hpysm" style="text-decoration: none;">
                        <span>View</span>
                    </a>
                </div>
            </div>
        `;
    });

    grid.innerHTML = html;

    if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.promptRejectShopItem = function(id, name) {
    if (window.showModal) {
        const safeName = window.escapeHTML(name || 'this item');
        window.showModal({
            title: 'Reject Shop Item',
            subtitle: `Item #${id} (${safeName})`,
            body: `
                <p style="font-family: 'Ubuntu', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Are you sure you want to reject this item? It will stay hidden from the shop.</p>
                <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">
                    <button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()">
                        <span>Cancel</span>
                    </button>
                    <button type="button" class="primary hpydng hpyinl hpysm" onclick="window.confirmRejectShopItem(${id}, event)">
                        <span>Reject</span>
                    </button>
                </div>
            `
        });
    } else {
        window.reviewAdminShopItem(id, 'reject');
    }
};

window.confirmRejectShopItem = function(id, event) {
    if (event) event.preventDefault();
    if (window.closeModal) window.closeModal();
    window.reviewAdminShopItem(id, 'reject');
};

window.reviewAdminShopItem = function(id, action) {
    const csrfInput = document.getElementById('admShopCsrf');

    const body = new URLSearchParams();
    if (csrfInput) body.set('csrf', csrfInput.value);

    fetch(`/admin/shop/${id}/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json'
        },
        body: body.toString()
    })
        .then(res => {
            if (!res.ok) throw new Error('Review failed');
            window.fetchAdminShopItems(currentAdminShopItemPage);
        })
        .catch(() => {});
};

window.fetchAdminBanners = function(page = 1) {
    currentAdminBannerPage = page;

    const grid = document.getElementById('admBannersGrid');
    if (!grid) return;

    fetch(`/api/v1/admin/banners?page=${page}&limit=12`)
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminBanners(data);
        })
        .catch(() => {});
};

window.adminBannerStyle = function(banner) {
    const a = '#' + (banner.color_a || '7423CB');
    const b = '#' + (banner.color_b || '912ADF');
    const angle = Math.min(360, Math.max(0, Number(banner.angle) || 135));
    if (banner.banner_type === 'gradient') return `background:linear-gradient(${angle}deg,${a},${b});`;
    if (banner.banner_type === 'image') return `background:url('/static/uploads/banners/${banner.id}.png') center/cover no-repeat;`;
    return `background:${a};`;
};

window.renderAdminBanners = function(data) {
    const grid = document.getElementById('admBannersGrid');
    const pgnTxt = document.getElementById('admBannersPgnTxt');
    const prevBtn = document.getElementById('admBannersPrevBtn');
    const nextBtn = document.getElementById('admBannersNextBtn');

    if (!grid) return;

    const page = data.page || 1;
    const totalPages = Math.max(1, data.total_pages || 1);

    if (!data.banners || data.banners.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No banners awaiting review</div>';
        if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
        if (prevBtn) prevBtn.disabled = (page <= 1);
        if (nextBtn) nextBtn.disabled = (page >= totalPages);
        window.updateAdminViewportHeight();
        return;
    }

    let html = '';
    data.banners.forEach(banner => {
        const safeOwner = window.escapeHTML(banner.username || '');
        const typeName = banner.banner_type === 'gradient' ? 'Gradient' : (banner.banner_type === 'image' ? 'Image' : 'Color');
        const style = window.adminBannerStyle(banner);

        let dateStr = '';
        if (banner.created_at) {
            const created = new Date(banner.created_at);
            dateStr = created.toLocaleString('en-US', {
                month: 'short', day: '2-digit', year: 'numeric',
                hour: '2-digit', minute: '2-digit', hour12: true
            });
        }

        html += `
            <div class="adm-card" data-id="${banner.id}">
                <div class="adm-cardhdr">
                    <h3 class="adm-cardttl"><i class="fa-solid fa-image"></i><span>${typeName} Banner</span></h3>
                    <span class="adm-badge adm-badge-image">${typeName}</span>
                </div>
                <div style="width:100%;aspect-ratio:16/5;border-radius:8px;border:1px solid #D4D4D4;background:#D4D4D4;box-sizing:border-box;${style}"></div>
                <div class="adm-rowlst">
                    <div class="adm-row"><span class="adm-lbl">Owner</span><span class="adm-val"><a href="/user/${banner.user_id}" hx-get="/user/${banner.user_id}" hx-target="body" hx-push-url="true" class="adm-owner-link">@${safeOwner}</a></span></div>
                    <div class="adm-row"><span class="adm-lbl">Submitted</span><span class="adm-val">${dateStr}</span></div>
                </div>
                <div class="adm-modbtns">
                    <button type="button" class="primary hpysuc hpyinl hpysm" onclick="window.reviewAdminBanner(${banner.id}, 'approve')">
                        <span>Approve</span>
                    </button>
                    <button type="button" class="primary hpydng hpyinl hpysm" onclick="window.promptRejectBanner(${banner.id}, '${safeOwner}')">
                        <span>Reject</span>
                    </button>
                    <a href="/user/${banner.user_id}" target="_blank" rel="noopener" class="primary hpyinf hpyinl hpysm" style="text-decoration: none;">
                        <span>View</span>
                    </a>
                </div>
            </div>
        `;
    });

    grid.innerHTML = html;

    if (pgnTxt) pgnTxt.textContent = `Page ${page} of ${totalPages}`;
    if (prevBtn) prevBtn.disabled = (page <= 1);
    if (nextBtn) nextBtn.disabled = (page >= totalPages);

    window.updateAdminViewportHeight();
};

window.promptRejectBanner = function(id, username) {
    if (window.showModal) {
        const safeOwner = window.escapeHTML(username || 'this user');
        window.showModal({
            title: 'Reject Profile Banner',
            subtitle: `Banner #${id} (@${safeOwner})`,
            body: `
                <p style="font-family: 'Ubuntu', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Are you sure you want to reject this banner? The user's current banner stays.</p>
                <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">
                    <button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()">
                        <span>Cancel</span>
                    </button>
                    <button type="button" class="primary hpydng hpyinl hpysm" onclick="window.confirmRejectBanner(${id}, event)">
                        <span>Reject</span>
                    </button>
                </div>
            `
        });
    } else {
        window.reviewAdminBanner(id, 'reject');
    }
};

window.confirmRejectBanner = function(id, event) {
    if (event) event.preventDefault();
    if (window.closeModal) window.closeModal();
    window.reviewAdminBanner(id, 'reject');
};

window.reviewAdminBanner = function(id, action) {
    const csrfInput = document.getElementById('admBannerCsrf');

    const body = new URLSearchParams();
    if (csrfInput) body.set('csrf', csrfInput.value);

    fetch(`/admin/banners/${id}/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json'
        },
        body: body.toString()
    })
        .then(res => {
            if (!res.ok) throw new Error('Review failed');
            window.fetchAdminBanners(currentAdminBannerPage);
        })
        .catch(() => {});
};

window.reviewAdminAsset = function(id, action, customNote) {
    const csrfInput = document.getElementById('admAssetsCsrf');
    const note = customNote !== undefined ? customNote : '';

    const body = new URLSearchParams();
    if (csrfInput) body.set('csrf', csrfInput.value);
    if (note) body.set('note', note);

    if (activeAdmin3DViewers.has(id)) {
        const viewer = activeAdmin3DViewers.get(id);
        viewer.destroy();
        activeAdmin3DViewers.delete(id);
    }

    fetch(`/admin/assets/${id}/${action}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json'
        },
        body: body.toString()
    })
        .then(res => {
            if (!res.ok) throw new Error('Review failed');
            window.fetchAdminAssets(currentAdminAssetPage);
            if (window.fetchAdminLogs) window.fetchAdminLogs(currentAdminLogsPage);
        })
        .catch(() => {});
};

window.fetchAdminGServers = function() {
    const grid = document.getElementById('admGsGrid');
    if (!grid) return;
    fetch('/api/v1/admin/gs')
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminGServers(data);
        })
        .catch(() => {});
    fetch('/api/v1/admin/gs/health')
        .then(res => res.json())
        .then(data => {
            if (!data || data.error) return;
            window.renderAdminGSHealth(data);
        })
        .catch(() => {});
};

window.renderAdminGSHealth = function(data) {
    const binEl = document.getElementById('admGsBin');
    const redisEl = document.getElementById('admGsRedis');
    const logsEl = document.getElementById('admGsLogs');
    if (binEl) {
        const bin = data.binary || {};
        if (bin.exists) {
            binEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-online"></span>Found</span>';
            binEl.title = (bin.path || '') + ' (' + (bin.size || 0) + ' bytes)';
        } else {
            binEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-offline"></span>Missing</span>';
            binEl.title = bin.path || '';
        }
    }
    if (redisEl) {
        const redis = data.redis || {};
        if (redis.ok) {
            redisEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-online"></span>' + window.escapeHTML(redis.addr || 'up') + '</span>';
        } else {
            redisEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-offline"></span>Down (memory fallback)</span>';
            redisEl.title = redis.error || '';
        }
    }
    if (logsEl) {
        const logs = data.logs || {};
        if (logs.writable) {
            logsEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-online"></span>Writable</span>';
            logsEl.title = logs.dir || '';
        } else {
            logsEl.innerHTML = '<span class="adm-stat"><span class="adm-ind adm-ind-offline"></span>Not writable</span>';
            logsEl.title = logs.dir || '';
        }
    }
};

window.renderAdminGServers = function(data) {
    const grid = document.getElementById('admGsGrid');
    if (!grid) return;
    const totalEl = document.getElementById('admGsTotal');
    const playersEl = document.getElementById('admGsPlayers');
    const portsEl = document.getElementById('admGsPorts');
    const servers = data.servers || [];
    if (totalEl) totalEl.textContent = data.total || 0;
    if (playersEl) playersEl.textContent = data.players || 0;
    if (portsEl) {
        const ports = data.ports || {};
        const claimed = ports.claimed || [];
        portsEl.textContent = (ports.min || '') + '-' + (ports.max || '') + ' (' + claimed.length + ' claimed)';
    }
    if (servers.length === 0) {
        grid.innerHTML = '<div class="frnemt adm-emt">No active gameservers</div>';
        window.updateAdminViewportHeight();
        return;
    }
    let html = '';
    servers.forEach(s => {
        const safeId = window.escapeHTML(s.id);
        const ind = s.alive ? 'adm-ind-online' : 'adm-ind-offline';
        const stateLabel = s.alive ? window.escapeHTML(s.status || '') : 'unreachable';
        let age = '';
        if (s.created_unix) {
            const secs = Math.max(0, Math.floor(Date.now() / 1000 - s.created_unix));
            age = formatUptimeJS(secs);
        }
        html +=
            '<div class="adm-card" data-id="' + safeId + '">' +
            '<div class="adm-cardhdr"><h3 class="adm-cardttl"><i class="fa-solid fa-server"></i><span>' + safeId + '</span></h3>' +
            '<span class="adm-stat"><span class="adm-ind ' + ind + '"></span>' + stateLabel + '</span></div>' +
            '<div class="adm-rowlst">' +
            '<div class="adm-row"><span class="adm-lbl">Game</span><span class="adm-val"><a href="/play/' + s.game_id + '" hx-get="/play/' + s.game_id + '" hx-target="body" hx-push-url="true" class="adm-owner-link">#' + s.game_id + '</a></span></div>' +
            '<div class="adm-row"><span class="adm-lbl">Address</span><span class="adm-val">' + window.escapeHTML(s.host || '') + ':' + s.port + '</span></div>' +
            '<div class="adm-row"><span class="adm-lbl">Players</span><span class="adm-val">' + s.player_count + ' / ' + s.max_players + '</span></div>' +
            (age ? '<div class="adm-row"><span class="adm-lbl">Uptime</span><span class="adm-val">' + window.escapeHTML(age) + '</span></div>' : '') +
            '</div>' +
            '<div class="adm-modbtns">' +
            '<button type="button" class="primary hpywrn hpyinl hpysm" onclick="window.drainGameserver(\'' + safeId + '\')"><span>Drain</span></button>' +
            '<button type="button" class="primary hpydng hpyinl hpysm" onclick="window.promptKillGameserver(\'' + safeId + '\')"><span>Kill</span></button>' +
            '</div></div>';
    });
    grid.innerHTML = html;
    window.updateAdminViewportHeight();
};

window.drainGameserver = function(id) {
    fetch('/api/v1/admin/gs/' + encodeURIComponent(id) + '/drain', {method: 'POST'})
        .then(() => { window.fetchAdminGServers(); })
        .catch(() => {});
};

window.promptKillGameserver = function(id) {
    if (window.showModal) {
        window.showModal({
            title: 'Kill Gameserver?',
            subtitle: id,
            body:
                '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">This winds the server down immediately and disconnects everyone on it. Empty servers shut down on their own; only kill servers that are stuck or misbehaving.</p>' +
                '<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:16px;">' +
                '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Cancel</span></button>' +
                '<button type="button" class="primary hpydng hpyinl hpysm" onclick="window.confirmKillGameserver(\'' + window.escapeHTML(id) + '\')"><span>Kill</span></button>' +
                '</div>'
        });
    } else {
        window.confirmKillGameserver(id);
    }
};

window.confirmKillGameserver = function(id) {
    if (window.closeModal) window.closeModal();
    fetch('/api/v1/admin/gs/' + encodeURIComponent(id) + '/kill', {method: 'POST'})
        .then(() => { window.fetchAdminGServers(); })
        .catch(() => {});
};

const initAdminPage = () => {
    window.destroyAllAdmin3DViewers();
    if (adminUptimeTimer) clearInterval(adminUptimeTimer);
    if (adminPollTimer) clearInterval(adminPollTimer);

    setTimeout(() => {
        window.updateAdminTabIndicator();
        window.updateAssetTabIndicator();
        window.updateShopItemTabIndicator();
        window.updateAdminViewportHeight();
    }, 50);
    setTimeout(() => {
        window.updateAdminTabIndicator();
        window.updateAssetTabIndicator();
        window.updateShopItemTabIndicator();
        window.updateAdminViewportHeight();
    }, 150);

    updateLiveUptime();
    adminUptimeTimer = setInterval(updateLiveUptime, 1000);
    adminPollTimer = setInterval(pollAdminStatus, 5000);
    const serversTab = document.getElementById('tab-servers');
    if (serversTab && serversTab.classList.contains('active') && window.fetchAdminGServers) {
        window.fetchAdminGServers();
    }
};

initAdminPage();

document.addEventListener('htmx:afterSettle', initAdminPage);
document.addEventListener('htmx:beforeTransition', () => {
    window.destroyAllAdmin3DViewers();
    if (adminUptimeTimer) clearInterval(adminUptimeTimer);
    if (adminPollTimer) clearInterval(adminPollTimer);
});

window.addEventListener('resize', () => {
    window.updateAdminTabIndicator();
    window.updateAssetTabIndicator();
    window.updateShopItemTabIndicator();
    window.updateAdminViewportHeight();
});
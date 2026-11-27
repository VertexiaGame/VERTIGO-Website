(function() {
    function esc(s) {
        if (window.escapeHTML) return window.escapeHTML(String(s));
        return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#039;');
    }



    function refreshTabIndicator() {
        if (window.updateItemTabIndicator) window.updateItemTabIndicator();
    }

    async function updateSpecialItemDetails(box) {
        const priceBlock = box.closest('.shpitm-priceblock');
        const market = document.querySelector('.shpitm-market');
        const tabs = document.querySelector('#shopItemTabs .prftablst');
        const activeTab = tabs?.querySelector('.shpitm-tabbtn.active');
        const url = new URL(window.location.href);
        if (activeTab) url.searchParams.set('tab', new URL(activeTab.href).searchParams.get('tab'));

        const response = await fetch(url, {cache: 'no-store'});
        if (!response.ok) throw new Error('Could not load updated item details.');
        const page = new DOMParser().parseFromString(await response.text(), 'text/html');
        const updatedPriceBlock = page.querySelector('.shpitm-priceblock');
        if (!priceBlock?.isConnected) return;
        if (!updatedPriceBlock) throw new Error('Could not load updated item details.');

        const meta = document.querySelector('.shpitm-metastrip');
        const updatedMeta = page.querySelector('.shpitm-metastrip');
        if (meta && updatedMeta) meta.replaceChildren(...updatedMeta.childNodes);
        for (const selector of ['.sbtnvrt .navtxt', '.sbtntix .navtxt', '#userVerticesCount']) {
            const updated = page.querySelector(selector);
            if (updated) document.querySelectorAll(selector).forEach(el => { el.textContent = updated.textContent; });
        }

        const resellerTab = '#shopItemTabs .shpitm-tabbtn[href$="?tab=resellers"]';
        const updatedResellerTab = page.querySelector(resellerTab);
        if (tabs?.isConnected && updatedResellerTab && !document.querySelector(resellerTab)) {
            const indicator = tabs.querySelector('#shopItemTabIndicator');
            indicator.after(updatedResellerTab);
            window.htmx?.process(updatedResellerTab);
            refreshTabIndicator();
        }

        const updatedMarket = page.querySelector('.shpitm-market');
        if (market?.isConnected && updatedMarket) {
            market.replaceWith(updatedMarket);
            window.htmx?.process(updatedMarket);
        }

        priceBlock.replaceWith(updatedPriceBlock);
        window.htmx?.process(updatedPriceBlock);
        return updatedPriceBlock.querySelector('#shpitmWearMsg');
    }

    window.purchaseItem = function(currency) {
        var box = document.getElementById('shpitmActions');
        var msg = document.getElementById('shpitmWearMsg');
        if (!box) return;
        var itemId = box.getAttribute('data-item-id');
        if (!itemId) return;
        currency = currency || '';
        var doPurchase = function() {
            if (msg) { msg.textContent = 'Purchasing...'; msg.className = 'shpitm-wearmsg'; }
            var btns = box.querySelectorAll('button');
            btns.forEach(function(b){ b.disabled = true; b.style.opacity = '0.6'; });
            var fd = new FormData();
            if (currency) fd.append('currency', currency);
            fetch('/api/v1/shop/purchase/' + encodeURIComponent(itemId), {method:'POST', body: fd, headers:{'HX-Request':'true'}})
                .then(function(res){ return res.json().then(function(j){ return {ok:res.ok, status:res.status, body:j}; }); })
                .then(async function(out){
                    if (!out.ok) throw new Error((out.body && out.body.error) || 'Purchase failed');
                    if (!box.isConnected) return;
                    box.setAttribute('data-is-owned', 'true');
                    box.innerHTML = '<button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.toggleItemWear(true)"><span>Wear</span></button>';
                    try {
                        if (typeof out.body.bucks !== 'undefined') {
                            var vEl = document.getElementById('userVerticesCount');
                            if (vEl) vEl.textContent = out.body.bucks;
                            document.querySelectorAll('.sbtnvrt .navtxt').forEach(function(el){ el.textContent = out.body.bucks; });
                        }
                        if (typeof out.body.bits !== 'undefined') {
                            document.querySelectorAll('.sbtntix .navtxt').forEach(function(el){ el.textContent = out.body.bits; });
                        }
                    } catch(e){}
                    var message = 'Purchased! You now own this item.';
                    if (box.dataset.isSpecial === 'true') {
                        try {
                            msg = await updateSpecialItemDetails(box);
                        } catch (error) {
                            message = 'Purchased! Updated stock and serial details could not be loaded.';
                        }
                    }
                    if (msg) { msg.textContent = message; msg.className = 'shpitm-wearmsg ok'; }
                    setTimeout(function(){ if(msg) msg.textContent=''; }, 3200);
                })
                .catch(function(err){
                    if (msg) { msg.textContent = err.message || 'Purchase failed.'; msg.className = 'shpitm-wearmsg err'; }
                    var btns2 = box.querySelectorAll('button');
                    btns2.forEach(function(b){ b.disabled = false; b.style.opacity = ''; });
                    setTimeout(function(){ if(msg && msg.classList.contains('err')) msg.textContent=''; }, 4200);
                });
        };
        if (currency === 'vertices' || currency === 'tickets') {
            var priceEl = null;
            var priceVal = 0;
            try {
                var vals = document.querySelectorAll('.shpitm-pricevals .shp-price-val span');
            } catch(e){}
            if (window.showModal) {
                var isVerts = currency === 'vertices';
                var curName = isVerts ? 'Vertices' : 'Tickets';
                var curIcon = isVerts ? '/static/useful/icons/verticec.png' : '/static/useful/icons/ticketc.png';
                var priceText = '';
                try {
                    var allVals = document.querySelectorAll('.shpitm-pricevals .shp-price-val');
                    allVals.forEach(function(row){
                        var img = row.querySelector('img');
                        if (!img) return;
                        var src = img.getAttribute('src') || '';
                        var want = isVerts ? 'verticec' : 'ticketc';
                        if (src.indexOf(want) !== -1) {
                            var sp = row.querySelector('span:last-child');
                            if (sp) priceText = sp.textContent.trim();
                        }
                    });
                } catch(e){}
                window.showModal({
                    title: 'Confirm Purchase',
                    body: '<div style="display:flex;flex-direction:column;gap:12px;">'
                        + '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">Are you sure you want to buy this item for <strong style="display:inline-flex;align-items:center;gap:4px;"><img src="'+curIcon+'" style="width:14px;height:14px;"> '+esc(priceText)+' '+curName+'</strong>?</p>'
                        + '<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:4px;">'
                        + '<button type="button" class="primary hpyerr hpyinl hpysm" onclick="window.closeModal()"><span>Cancel</span></button>'
                        + '<button type="button" class="primary hpysuc hpyinl hpysm" id="confirmPurchaseBtn"><span>Confirm</span></button>'
                        + '</div></div>'
                });
                setTimeout(function(){
                    var cBtn = document.getElementById('confirmPurchaseBtn');
                    if (cBtn) cBtn.addEventListener('click', function(){
                        window.closeModal();
                        doPurchase();
                    });
                }, 80);
                return;
            }
        }
        if (currency === 'free') {
            doPurchase();
            return;
        }
        doPurchase();
    };

    window.submitShopResale = function(event) {
        event.preventDefault();
        const form = event.currentTarget;
        const url = event.submitter?.getAttribute('formaction') || form.action;
        const status = document.getElementById('shpitmResaleStatus');
        const itemActions = document.getElementById('shpitmActions');
        const submit = async () => {
            const buttons = form.querySelectorAll('button');
            buttons.forEach(button => { button.disabled = true; });
            status.textContent = 'Saving…';
            status.className = 'shpitm-wearmsg';
            try {
                const response = await fetch(url, {
                    method: 'POST',
                    body: new FormData(form),
                    headers: { Accept: 'application/json' }
                });
                const result = await response.json();
                if (!response.ok || !result.success) throw new Error(result.error);
                if (!itemActions?.isConnected) return;
                try {
                    const notice = await updateSpecialItemDetails(itemActions);
                    if (notice) {
                        notice.textContent = form.dataset.serial ? 'Purchased! You now own this item.' : 'Listing updated.';
                        notice.className = 'shpitm-wearmsg ok';
                    }
                } catch (error) {
                    status.textContent = 'Transaction completed. Updated item details could not be loaded.';
                    status.className = 'shpitm-wearmsg ok';
                    buttons.forEach(button => { button.disabled = false; });
                }
            } catch (error) {
                status.textContent = error.message || 'The transaction could not be completed.';
                status.className = 'shpitm-wearmsg err';
                status.scrollIntoView({ block: 'nearest' });
                buttons.forEach(button => { button.disabled = false; });
            }
        };
        if (form.dataset.serial) {
            const body = document.createElement('div');
            body.innerHTML = '<p></p><div class="shpitm-market-fields">'
                + '<button type="button" class="primary hpyinf hpyinl hpysm"><span>Cancel</span></button>'
                + '<button type="button" class="primary hpysuc hpyinl hpysm"><span>Confirm Purchase</span></button>'
                + '</div>';
            body.querySelector('p').textContent = 'Buy serial #' + form.dataset.serial + ' for ' + form.dataset.price + ' Vertices?';
            const [cancel, confirm] = body.querySelectorAll('button');
            cancel.onclick = () => window.closeModal();
            confirm.onclick = () => {
                window.closeModal();
                submit();
            };
            window.showModal({ title: 'Confirm Purchase', body });
            cancel.focus();
        } else {
            submit();
        }
        return false;
    };

    window.toggleItemWear = function(wear) {
        var box = document.getElementById('shpitmActions');
        var msg = document.getElementById('shpitmWearMsg');
        if (!box) return;
        var itemId = box.getAttribute('data-item-id');
        var itemType = box.getAttribute('data-item-type');
        if (!itemId || !itemType) return;
        if (msg) { msg.textContent = wear ? 'Equipping...' : 'Unequipping...'; msg.className = 'shpitm-wearmsg'; }
        const wearButton = box.querySelector('button');
        if (wearButton?.disabled) return;
        if (wearButton) wearButton.disabled = true;
        var url = wear ? '/avatar/wear' : '/avatar/unwear';
        var fd = new FormData();
        fd.append('type', itemType);
        fd.append('id', itemId);
        fetch(url, {method:'POST', body: fd, headers:{'HX-Request':'true'}})
            .then(function(res){ return res.json().then(function(j){ return {ok:res.ok, body:j}; }); })
            .then(function(out){
                if (!out.ok) throw new Error((out.body && out.body.error) || 'Failed');
                if (!box.isConnected) return;
                box.dataset.isEquipped = String(wear);
                const button = box.querySelector('button');
                if (button) {
                    button.classList.toggle('hpydng', wear);
                    button.classList.toggle('hpyprim', !wear);
                    button.querySelector('span').textContent = wear ? 'Take Off' : 'Wear';
                    button.onclick = () => window.toggleItemWear(!wear);
                }
                if (msg) {
                    msg.textContent = wear ? 'Equipped!' : 'Unequipped!';
                    msg.className = 'shpitm-wearmsg ok';
                }
            })
            .catch(function(err){
                if (msg) { msg.textContent = err.message || 'Failed to update.'; msg.className = 'shpitm-wearmsg err'; }
            })
            .finally(function(){
                if (wearButton) wearButton.disabled = false;
            });
    };

    class ShopItemViewer {
        constructor(container, itemType, itemId) {
            this.container = container;
            this.itemType = itemType;
            this.itemId = itemId;
            this.isDestroyed = false;
            this.animId = null;
            this.isDragging = false;
            this.pointer = { x: 0, y: 0 };
            this.spherical = { radius: 3.6, theta: 0.5, phi: 0.35 };
            this.targetRadius = 3.6;
            this.minRadius = 1.2;
            this.maxRadius = 8.0;
            this.target = null;
            this.init();
        }

        meshUrls() {
            return [
                '/assets/uploads/shop/' + this.itemType + '/' + this.itemId + '.obj',
                '/assets/shop/' + this.itemType + '/' + this.itemId + '.obj'
            ];
        }

        textureUrls() {
            return [
                '/assets/uploads/shop/' + this.itemType + '/' + this.itemId + '.png',
                '/assets/shop/' + this.itemType + '/' + this.itemId + '.png'
            ];
        }

        async init() {
            this.container.innerHTML = '';
            const THREE = window.THREE || (await import('three'));
            const { OBJLoader } = await import('three/addons/loaders/OBJLoader.js');
            const { loadSolidOBJ } = await import('/static/js/three/objsolid.js');
            this.loadSolidOBJ = loadSolidOBJ;
            if (this.isDestroyed) return;
            this.THREE = THREE;
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
            this.spinner.innerHTML = '<svg class="dmspn" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">'
                + '<rect class="sq sq-1" x="43" y="7" width="14" height="14"/>'
                + '<rect class="sq sq-2" x="25" y="25" width="14" height="14"/>'
                + '<rect class="sq sq-3" x="7" y="43" width="14" height="14"/>'
                + '<rect class="sq sq-4" x="25" y="61" width="14" height="14"/>'
                + '<rect class="sq sq-5" x="43" y="79" width="14" height="14"/>'
                + '<rect class="sq sq-6" x="61" y="61" width="14" height="14"/>'
                + '<rect class="sq sq-7" x="79" y="43" width="14" height="14"/>'
                + '<rect class="sq sq-8" x="61" y="25" width="14" height="14"/>'
                + '</svg>';
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
            const THREE = this.THREE;
            let model = null;
            const meshCandidates = this.meshUrls();
            for (let i = 0; i < meshCandidates.length; i++) {
                try {
                    model = await this.loadSolidOBJ(this.OBJLoader, meshCandidates[i]);
                    if (model) break;
                } catch (e) {}
            }
            if (!model) throw new Error('Failed to load model');
            const textureLoader = new THREE.TextureLoader();
            let tex = null;
            const texCandidates = this.textureUrls();
            for (let i = 0; i < texCandidates.length; i++) {
                try {
                    tex = await textureLoader.loadAsync(texCandidates[i]);
                    if (tex) break;
                } catch (e) {}
            }
            if (tex && THREE.SRGBColorSpace) {
                tex.colorSpace = THREE.SRGBColorSpace;
            }
            const mat = new THREE.MeshStandardMaterial({
                color: tex ? new THREE.Color(0xffffff) : new THREE.Color(0xd6d6d6),
                map: tex || null,
                roughness: 0.6,
                metalness: 0.0,
                transparent: true
            });
            model.traverse(function(child) {
                if (child.isMesh) {
                    child.material = mat;
                    child.castShadow = true;
                    child.receiveShadow = true;
                    if (child.geometry && !child.geometry.attributes.normal) child.geometry.computeVertexNormals();
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
            const self = this;
            this.animId = requestAnimationFrame(function() { self.animate(); });
            if (!this.isDragging) {
                this.spherical.theta += 0.006;
            }
            this.updateCamera();
            this.renderer.render(this.scene, this.camera);
        }

        bindEvents() {
            this.activePointers = new Map();
            this.initialPinchDistance = null;
            const self = this;
            this.boundPointerDown = function(e) {
                if (e.cancelable && e.pointerType === 'touch') {
                    try { e.preventDefault(); } catch (_) {}
                }
                try { self.container.setPointerCapture(e.pointerId); } catch (_) {}
                self.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
                if (self.activePointers.size === 1) {
                    self.isDragging = true;
                    self.pointer = { x: e.clientX, y: e.clientY };
                } else if (self.activePointers.size === 2) {
                    self.isDragging = false;
                    const pts = Array.from(self.activePointers.values());
                    self.initialPinchDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
                }
            };
            this.boundPointerMove = function(e) {
                if (!self.activePointers.has(e.pointerId)) return;
                if (e.cancelable && e.pointerType === 'touch') {
                    try { e.preventDefault(); } catch (_) {}
                }
                self.activePointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
                if (self.activePointers.size === 1 && self.isDragging) {
                    const dx = e.clientX - self.pointer.x;
                    const dy = e.clientY - self.pointer.y;
                    self.pointer = { x: e.clientX, y: e.clientY };
                    self.spherical.theta -= dx * 0.008;
                    self.spherical.phi += dy * 0.008;
                    self.spherical.phi = Math.max(-1.45, Math.min(1.45, self.spherical.phi));
                } else if (self.activePointers.size === 2 && self.initialPinchDistance !== null) {
                    const pts = Array.from(self.activePointers.values());
                    const currentDistance = Math.hypot(pts[0].x - pts[1].x, pts[0].y - pts[1].y);
                    if (currentDistance > 0) {
                        const deltaDistance = self.initialPinchDistance - currentDistance;
                        self.targetRadius = Math.max(self.minRadius, Math.min(self.maxRadius, self.targetRadius + deltaDistance * 0.01));
                        self.initialPinchDistance = currentDistance;
                    }
                }
            };
            this.boundPointerUp = function(e) {
                self.activePointers.delete(e.pointerId);
                if (self.activePointers.size < 2) {
                    self.initialPinchDistance = null;
                }
                if (self.activePointers.size === 1) {
                    self.isDragging = true;
                    const p = Array.from(self.activePointers.values())[0];
                    self.pointer = { x: p.x, y: p.y };
                } else if (self.activePointers.size === 0) {
                    self.isDragging = false;
                }
                try { self.container.releasePointerCapture(e.pointerId); } catch (_) {}
            };
            this.boundWheel = function(e) {
                e.preventDefault();
                self.targetRadius = Math.max(self.minRadius, Math.min(self.maxRadius, self.targetRadius + e.deltaY * 0.004));
            };
            this.boundContextMenu = function(e) { e.preventDefault(); };
            this.container.addEventListener('pointerdown', this.boundPointerDown, { passive: false });
            this.container.addEventListener('pointermove', this.boundPointerMove, { passive: false });
            this.container.addEventListener('pointerup', this.boundPointerUp);
            this.container.addEventListener('pointercancel', this.boundPointerUp);
            this.container.addEventListener('contextmenu', this.boundContextMenu);
            this.container.addEventListener('wheel', this.boundWheel, { passive: false });
        }

        initResize() {
            const self = this;
            if (window.ResizeObserver) {
                this.ro = new ResizeObserver(function(entries) {
                    for (const entry of entries) {
                        const w = entry.contentRect.width;
                        const h = entry.contentRect.height;
                        if (w > 0 && h > 0 && self.renderer && self.camera) {
                            self.camera.aspect = w / h;
                            self.camera.updateProjectionMatrix();
                            self.renderer.setSize(w, h);
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
            if (this.container) {
                this.container.removeEventListener('pointerdown', this.boundPointerDown);
                this.container.removeEventListener('pointermove', this.boundPointerMove);
                this.container.removeEventListener('pointerup', this.boundPointerUp);
                this.container.removeEventListener('pointercancel', this.boundPointerUp);
                this.container.removeEventListener('contextmenu', this.boundContextMenu);
                this.container.removeEventListener('wheel', this.boundWheel);
            }
            if (this.scene) {
                this.scene.traverse(function(obj) {
                    if (obj.geometry) obj.geometry.dispose();
                    if (obj.material) {
                        if (Array.isArray(obj.material)) {
                            obj.material.forEach(function(m) {
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
            if (this.container) {
                this.container.innerHTML = '';
            }
        }
    }

    let activeShopItemViewer = null;

    window.toggleShopItem3D = function(btn) {
        const renderCrd = btn.closest('.shpitm-rendcrd') || btn.closest('.adm-rendcrd');
        if (!renderCrd) return;
        const img = renderCrd.querySelector('.adm-rendimg');
        const wrap = renderCrd.querySelector('.adm-3d-canvas-wrap');
        const label = btn.querySelector('span');
        if (activeShopItemViewer) {
            activeShopItemViewer.destroy();
            activeShopItemViewer = null;
            if (wrap) wrap.remove();
            if (img) img.style.display = '';
            btn.classList.remove('active');
            if (label) label.textContent = '3D View';
            return;
        }
        if (img) img.style.display = 'none';
        const fresh = document.createElement('div');
        fresh.className = 'adm-3d-canvas-wrap';
        renderCrd.appendChild(fresh);
        btn.classList.add('active');
        if (label) label.textContent = '2D View';
        activeShopItemViewer = new ShopItemViewer(fresh, btn.dataset.itemType, btn.dataset.itemId);
    };

    window.destroyShopItem3D = function() {
        if (activeShopItemViewer) {
            activeShopItemViewer.destroy();
            activeShopItemViewer = null;
        }
    };

    if (!window.__shopItem3DNavBound) {
        window.__shopItem3DNavBound = true;
        document.addEventListener('htmx:beforeTransition', function() {
            window.destroyShopItem3D();
        });
    }

    function initItem() {
        refreshTabIndicator();
        var bar = document.getElementById('shpitmBarSearch');
        if (bar && !bar._itemBound) {
            bar._itemBound = true;
            bar.addEventListener('keydown', function(e){
                if (e.key === 'Enter') {
                    e.preventDefault();
                    window.location.href = '/shop?q=' + encodeURIComponent(bar.value.trim());
                }
            });
        }
        document.querySelectorAll('.avtldimg').forEach(function(img){
            if (img.complete) img.classList.add('ld');
            else {
                img.addEventListener('load', function(){ img.classList.add('ld'); });
                img.addEventListener('error', function(){ img.classList.add('ld'); });
            }
        });
        window.addEventListener('resize', refreshTabIndicator);
    }

    document.addEventListener('DOMContentLoaded', initItem);
    document.addEventListener('htmx:afterSettle', initItem);
    initItem();
})();

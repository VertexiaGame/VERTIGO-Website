window.createType = 'shop';
window.createCat = 'hat';
window.assetCategory = 'image';

var ASSET_TYPE_INFO = {
    image: {
        name: 'Image',
        accept: 'image/png,image/jpeg,image/webp,image/gif',
        exts: ['png', 'jpg', 'jpeg', 'webp', 'gif'],
        max: 8 * 1024 * 1024,
        formats: 'PNG, JPG, WEBP, GIF',
        limit: 'Up to 8 MB',
        msg: 'Choose a PNG, JPG, WEBP or GIF'
    },
    mesh: {
        name: 'Mesh',
        accept: '.glb,.obj',
        exts: ['glb', 'obj'],
        max: 25 * 1024 * 1024,
        formats: 'GLB, OBJ',
        limit: 'Up to 25 MB',
        msg: 'Choose a GLB or OBJ'
    },
    sound: {
        name: 'Sound',
        accept: '.mp3,.wav,.ogg',
        exts: ['mp3', 'wav', 'ogg'],
        max: 15 * 1024 * 1024,
        formats: 'MP3, WAV, OGG',
        limit: 'Up to 15 MB',
        msg: 'Choose an MP3, WAV or OGG'
    }
};

var CRT_CAT_NAMES = {
    hat: 'Hat',
    face: 'Face',
    shirt: 'Shirt',
    tshirt: 'T-Shirt',
    pants: 'Pants',
    tool: 'Tool'
};

function crtEsc(str) {
    if (window.escapeHTML) return window.escapeHTML(String(str));
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

window.updateCreateViewportHeight = function() {
    var viewport = document.querySelector('.crtvwp');
    var active = document.querySelector('.crtvwp > .settabcntnt.active');
    if (!viewport || !active) return;
    viewport.style.height = active.offsetHeight + 'px';
    if (window.createHeightTimeout) clearTimeout(window.createHeightTimeout);
    window.createHeightTimeout = setTimeout(function() {
        if (viewport) viewport.style.height = '';
    }, 350);
};

window.switchCreateType = function(type, event) {
    if (window.crtAssetUploadXhr || window.crtShopUploadXhr || window.crtGameUploadXhr) {
        window.createShowError('Please wait for the upload to finish before switching tabs.');
        return;
    }
    var tab = event ? event.currentTarget : null;
    if (!tab || tab.classList.contains('active')) return;

    var current = document.querySelector('.crtvwp > .settabcntnt.active');
    var target = document.getElementById('crt-' + type);
    if (!target) return;

    document.querySelectorAll('.crttypebtn').forEach(function(btn) {
        btn.classList.remove('active');
    });
    tab.classList.add('active');

    var viewport = document.querySelector('.crtvwp');
    var currentHeight = current ? current.offsetHeight : 0;
    if (viewport) {
        viewport.style.height = currentHeight + 'px';
        viewport.offsetHeight;
    }

    if (current) current.classList.remove('active');
    target.classList.add('active');

    window.createType = type;

    if (type === 'shop') {
        window.updateShopCategoryIndicator();
    } else if (type === 'asset') {
        window.updateAssetCategoryIndicator();
    }

    var newHeight = target.offsetHeight;
    if (viewport) {
        viewport.style.height = newHeight + 'px';
    }

    if (window.createHeightTimeout) clearTimeout(window.createHeightTimeout);
    window.createHeightTimeout = setTimeout(function() {
        if (viewport) viewport.style.height = '';
    }, 350);

    window.updateCreateUrl();
};

window.updateShopCategoryIndicator = function() {
    var tabs = document.querySelector('.crtshoptabs');
    if (!tabs) return;
    var indicator = tabs.querySelector('.tabind');
    var activeTab = tabs.querySelector('.tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.switchShopCategory = function(cat, event) {
    if (window.crtShopUploadXhr || window.crtAssetUploadXhr || window.crtGameUploadXhr) {
        window.createShowError('Please wait for the upload to finish before switching tabs.');
        return;
    }
    var tab = event ? event.currentTarget : null;
    if (!tab || tab.classList.contains('active')) return;

    var tabs = document.querySelector('.crtshoptabs');
    if (tabs) {
        tabs.querySelectorAll('.tabbtn').forEach(function(t) {
            t.classList.remove('active');
        });
        tab.classList.add('active');
    }

    window.createCat = cat;
    window.updateShopCategoryIndicator();

    var viewport = document.querySelector('.crtvwp');
    var shopPane = document.getElementById('crt-shop');
    var currentHeight = shopPane ? shopPane.offsetHeight : 0;
    if (viewport) {
        viewport.style.height = currentHeight + 'px';
        viewport.offsetHeight;
    }

    var display = CRT_CAT_NAMES[cat] || cat;
    var modelField = document.getElementById('crtShopModelFld');
    if (modelField) {
        modelField.classList.toggle('visible', cat === 'hat' || cat === 'tool');
    }

    var titleEl = document.getElementById('crtShopTitle');
    if (titleEl) titleEl.textContent = 'Create ' + display;
    var catNameEl = document.getElementById('crtShopCatName');
    if (catNameEl) catNameEl.textContent = display;
    var input = document.getElementById('crtShopCatInput');
    if (input) input.value = cat;

    if (window.setCreateCategory) window.setCreateCategory(cat);
    if (window.refreshCreatePreview) window.refreshCreatePreview();
    window.updateShopPreviewName();

    var newHeight = shopPane ? shopPane.offsetHeight : 0;
    if (viewport) {
        viewport.style.height = newHeight + 'px';
    }

    if (window.createHeightTimeout) clearTimeout(window.createHeightTimeout);
    window.createHeightTimeout = setTimeout(function() {
        if (viewport) viewport.style.height = '';
    }, 350);

    window.updateCreateUrl();
};

window.updateShopPreviewName = function() {
    var nameInput = document.getElementById('crtShopName');
    var catName = CRT_CAT_NAMES[window.createCat] || window.createCat;
    var nameEl = document.getElementById('crtShopPreviewName');
    if (!nameEl) return;
    var value = nameInput ? nameInput.value.trim() : '';
    nameEl.textContent = value || ('Untitled ' + catName);
};

window.handleShopFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    var msgEl = document.getElementById('crtShopFileMsg');
    var nameEl = document.getElementById('crtShopFileName');
    var previewImg = document.getElementById('crtShopPreviewImg');
    if (window.setCreateTextureFile) window.setCreateTextureFile(file);
    else window.pendingCreateTextureFile = file;
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';
    if (previewImg) {
        previewImg.classList.remove('ld');
        var url = URL.createObjectURL(file);
        previewImg.onload = function() {
            previewImg.classList.add('ld');
            URL.revokeObjectURL(url);
        };
        previewImg.src = url;
    }
    if (window.refreshCreatePreview) window.refreshCreatePreview(true);
};

window.updateAssetCategoryIndicator = function() {
    var tabs = document.querySelector('.crtassettabs');
    if (!tabs) return;
    var indicator = tabs.querySelector('.tabind');
    var activeTab = tabs.querySelector('.tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.updateAssetTab = function() {
    var info = ASSET_TYPE_INFO[window.assetCategory] || ASSET_TYPE_INFO.image;

    var titleEl = document.getElementById('crtAssetTitle');
    if (titleEl) titleEl.textContent = 'Create ' + info.name;

    var typeInput = document.getElementById('crtAssetTypeInput');
    if (typeInput) typeInput.value = window.assetCategory;

    var fileInput = document.getElementById('crtAssetFile');
    if (fileInput) fileInput.accept = info.accept;

    var msgEl = document.getElementById('crtAssetFileMsg');
    if (msgEl) msgEl.textContent = info.msg;

    var tipEl = document.getElementById('crtAssetFileTip');
    if (tipEl) tipEl.textContent = info.formats;

    var fmtEl = document.getElementById('crtAssetFormats');
    if (fmtEl) fmtEl.textContent = info.formats;

    var limEl = document.getElementById('crtAssetLimit');
    if (limEl) limEl.textContent = info.limit;

    var nameEl = document.getElementById('crtAssetFileName');
    if (nameEl) nameEl.textContent = '';

    if (fileInput) fileInput.value = '';

    var texFld = document.getElementById('crtAssetTextureFld');
    if (texFld) {
        texFld.style.display = window.assetCategory === 'mesh' ? 'flex' : 'none';
        var texInput = document.getElementById('crtAssetTextureFile');
        if (texInput) texInput.value = '';
        var texNameEl = document.getElementById('crtAssetTextureName');
        if (texNameEl) texNameEl.textContent = '';
        var texMsgEl = document.getElementById('crtAssetTextureMsg');
        if (texMsgEl) texMsgEl.textContent = 'Choose a PNG or JPEG texture';
    }

    var badgeEl = document.getElementById('crtAssetPreviewBadge');
    if (badgeEl) {
        badgeEl.textContent = info.name;
        badgeEl.className = 'avtitmbdg adm-badge-' + window.assetCategory;
    }

    var previewTypeEl = document.getElementById('crtAssetPreviewType');
    if (previewTypeEl) {
        previewTypeEl.textContent = info.name + ' Asset';
    }

    var imgEl = document.getElementById('crtAssetPreviewImg');
    var audioIcon = document.getElementById('crtAssetPreviewAudioIcon');
    var meshIcon = document.getElementById('crtAssetPreviewMeshIcon');
    var imgWrap = document.getElementById('crtAssetImgWrap');

    if (imgEl && audioIcon && meshIcon && imgWrap) {
        if (window.assetCategory === 'sound') {
            imgEl.style.display = 'none';
            meshIcon.style.display = 'none';
            audioIcon.style.display = 'block';
            imgWrap.classList.add('avtitmico');
        } else if (window.assetCategory === 'mesh') {
            imgEl.style.display = 'none';
            audioIcon.style.display = 'none';
            meshIcon.style.display = 'block';
            imgWrap.classList.add('avtitmico');
        } else {
            audioIcon.style.display = 'none';
            meshIcon.style.display = 'none';
            imgEl.style.display = 'block';
            imgEl.src = '/static/useful/temp/pfp.png';
            imgWrap.classList.remove('avtitmico');
        }
    }

    window.updateAssetPreviewName();
};

window.updateAssetPreviewName = function() {
    var nameInput = document.getElementById('crtAssetName');
    var info = ASSET_TYPE_INFO[window.assetCategory] || ASSET_TYPE_INFO.image;
    var nameEl = document.getElementById('crtAssetPreviewName');
    if (!nameEl) return;
    var value = nameInput ? nameInput.value.trim() : '';
    nameEl.textContent = value || ('Untitled ' + info.name);
};

window.switchAssetCategory = function(cat, event) {
    if (window.crtAssetUploadXhr || window.crtShopUploadXhr || window.crtGameUploadXhr) {
        window.createShowError('Please wait for the upload to finish before switching tabs.');
        return;
    }
    var tab = event ? event.currentTarget : null;
    if (!tab || tab.classList.contains('active')) return;

    var tabs = document.querySelector('.crtassettabs');
    if (tabs) {
        tabs.querySelectorAll('.tabbtn').forEach(function(t) {
            t.classList.remove('active');
        });
        tab.classList.add('active');
    }

    window.assetCategory = cat;
    window.updateAssetCategoryIndicator();

    var viewport = document.querySelector('.crtvwp');
    var assetPane = document.getElementById('crt-asset');
    var currentHeight = assetPane ? assetPane.offsetHeight : 0;
    if (viewport) {
        viewport.style.height = currentHeight + 'px';
        viewport.offsetHeight;
    }

    window.updateAssetTab();

    var newHeight = assetPane ? assetPane.offsetHeight : 0;
    if (viewport) {
        viewport.style.height = newHeight + 'px';
    }

    if (window.createHeightTimeout) clearTimeout(window.createHeightTimeout);
    window.createHeightTimeout = setTimeout(function() {
        if (viewport) viewport.style.height = '';
    }, 350);

    window.updateCreateUrl();
};

window.handleAssetFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;

    var info = ASSET_TYPE_INFO[window.assetCategory] || ASSET_TYPE_INFO.image;
    var ext = (file.name.split('.').pop() || '').toLowerCase();

    if (info.exts.indexOf(ext) === -1) {
        input.value = '';
        window.createShowError('That file type is not supported for ' + info.name + ' assets. Supported formats: ' + info.formats + '.');
        return;
    }
    if (file.size > info.max) {
        input.value = '';
        window.createShowError(info.name + ' assets must be ' + info.limit.toLowerCase() + '.');
        return;
    }

    var nameEl = document.getElementById('crtAssetFileName');
    var msgEl = document.getElementById('crtAssetFileMsg');
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';

    if (window.assetCategory === 'image') {
        var previewImg = document.getElementById('crtAssetPreviewImg');
        var audioIcon = document.getElementById('crtAssetPreviewAudioIcon');
        var meshIcon = document.getElementById('crtAssetPreviewMeshIcon');
        var imgWrap = document.getElementById('crtAssetImgWrap');
        if (previewImg && imgWrap) {
            if (audioIcon) audioIcon.style.display = 'none';
            if (meshIcon) meshIcon.style.display = 'none';
            previewImg.style.display = 'block';
            imgWrap.classList.remove('avtitmico');
            previewImg.classList.remove('ld');
            var url = URL.createObjectURL(file);
            previewImg.onload = function() {
                previewImg.classList.add('ld');
                URL.revokeObjectURL(url);
            };
            previewImg.src = url;
        }
    }
};

window.handleAssetTextureFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;

    var ext = (file.name.split('.').pop() || '').toLowerCase();
    var validExts = ['png', 'jpg', 'jpeg', 'webp', 'gif'];
    if (validExts.indexOf(ext) === -1) {
        input.value = '';
        window.createShowError('The texture must be a PNG, JPG, WEBP, or GIF image.');
        return;
    }

    var nameEl = document.getElementById('crtAssetTextureName');
    var msgEl = document.getElementById('crtAssetTextureMsg');
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different texture';
};

window.updateGamePreviewName = function() {
    var nameInput = document.getElementById('crtGameName');
    var nameEl = document.getElementById('crtGamePreviewName');
    if (!nameEl) return;
    var value = nameInput ? nameInput.value.trim() : '';
    nameEl.textContent = value || 'Untitled Game';
};

window.checkGameImageFile = function(file, kind) {
    if (!file) return true;
    var ext = (file.name.split('.').pop() || '').toLowerCase();
    if (ext !== 'png' && ext !== 'jpg' && ext !== 'jpeg') {
        window.createShowError('The ' + kind + ' must be a PNG or JPEG image.');
        return false;
    }
    if (file.size > 8 * 1024 * 1024) {
        window.createShowError('The ' + kind + ' must be up to 8 MB.');
        return false;
    }
    return true;
};

window.handleGameFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    if (!window.checkGameImageFile(file, 'icon')) {
        input.value = '';
        return;
    }
    var nameEl = document.getElementById('crtGameFileName');
    var msgEl = document.getElementById('crtGameFileMsg');
    var previewImg = document.getElementById('crtGamePreviewImg');
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';
    if (previewImg) {
        previewImg.classList.remove('ld');
        var url = URL.createObjectURL(file);
        previewImg.onload = function() {
            previewImg.classList.add('ld');
            URL.revokeObjectURL(url);
        };
        previewImg.src = url;
    }
};

window.getGameThumbFiles = function(input) {
    var files = [];
    if (input && input.files) {
        for (var i = 0; i < input.files.length; i++) files.push(input.files[i]);
    }
    return files;
};

window.checkGameThumbFiles = function(files) {
    if (files.length > 5) {
        window.createShowError('Up to 5 thumbnails allowed.');
        return false;
    }
    for (var i = 0; i < files.length; i++) {
        if (!window.checkGameImageFile(files[i], 'thumbnail')) return false;
    }
    return true;
};

window.gameThumbPicked = [];
window.gameThumbPickURLs = [];
window.gameThumbDragIndex = -1;
window.gameThumbPreviewURL = null;
window.gameThumbSeenKeys = {};
window.gameThumbBannerKey = null;

window.gameThumbKey = function(f) {
    return f.name + '|' + f.size + '|' + f.lastModified;
};

window.gameThumbReducedMotion = function() {
    try {
        return window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    } catch (e) {}
    return false;
};

window.clearGameThumbPickURLs = function() {
    window.gameThumbPickURLs.forEach(function(u) {
        try { URL.revokeObjectURL(u); } catch (e) {}
    });
    window.gameThumbPickURLs = [];
};

window.syncGameThumbInput = function(input) {
    if (!input) return;
    try {
        var dt = new DataTransfer();
        window.gameThumbPicked.forEach(function(f) { dt.items.add(f); });
        input.files = dt.files;
    } catch (e) {}
};

window.updateGameThumbCount = function() {
    var countEl = document.getElementById('crtGameThumbCount');
    if (countEl) countEl.textContent = window.gameThumbPicked.length + '/5';
};

window.swapGameThumbBanner = function(src, url) {
    var thumbPreview = document.getElementById('crtGameThumbPreview');
    if (!thumbPreview) {
        if (url) {
            try { URL.revokeObjectURL(url); } catch (e) {}
        }
        if (window.gameThumbPreviewURL && window.gameThumbPreviewURL !== url) {
            try { URL.revokeObjectURL(window.gameThumbPreviewURL); } catch (e) {}
        }
        window.gameThumbPreviewURL = null;
        return;
    }
    if (window.gameThumbPreviewURL && window.gameThumbPreviewURL !== url) {
        try { URL.revokeObjectURL(window.gameThumbPreviewURL); } catch (e) {}
    }
    window.gameThumbPreviewURL = url || null;
    thumbPreview.onload = function() {
        if (url) {
            try { URL.revokeObjectURL(url); } catch (e) {}
            if (window.gameThumbPreviewURL === url) window.gameThumbPreviewURL = null;
        }
        thumbPreview.style.opacity = '1';
    };
    thumbPreview.onerror = function() {
        thumbPreview.style.opacity = '1';
    };
    thumbPreview.style.opacity = '0';
    thumbPreview.src = src;
};

window.updateGameThumbBanner = function() {
    var thumbPreview = document.getElementById('crtGameThumbPreview');
    if (!thumbPreview) return;
    if (window.gameThumbPicked.length === 0) {
        if (document.getElementById('crtGameThumbsStrip')) return;
        if (window.gameThumbBannerKey === null) return;
        window.gameThumbBannerKey = null;
        window.swapGameThumbBanner('/static/useful/temp/pfp.png', null);
        return;
    }
    var key = window.gameThumbKey(window.gameThumbPicked[0]);
    if (key === window.gameThumbBannerKey) return;
    window.gameThumbBannerKey = key;
    var burl = URL.createObjectURL(window.gameThumbPicked[0]);
    window.swapGameThumbBanner(burl, burl);
};

window.captureGameThumbFlip = function(strip) {
    var rects = {};
    if (!strip || window.gameThumbReducedMotion()) return null;
    strip.querySelectorAll('.crtthumbs-pick').forEach(function(c) {
        if (c.dataset.filekey) rects[c.dataset.filekey] = c.getBoundingClientRect();
    });
    return rects;
};

window.playGameThumbFlip = function(rects) {
    if (!rects) return;
    var strip = document.getElementById('crtGameThumbPicked');
    if (!strip) return;
    strip.querySelectorAll('.crtthumbs-pick').forEach(function(c) {
        var old = rects[c.dataset.filekey];
        if (!old) return;
        var now = c.getBoundingClientRect();
        var dx = old.left - now.left;
        var dy = old.top - now.top;
        if (dx === 0 && dy === 0) return;
        try {
            c.animate([
                {transform: 'translate(' + dx + 'px,' + dy + 'px)'},
                {transform: 'translate(0px,0px)'}
            ], {duration: 240, easing: 'cubic-bezier(0.25, 1, 0.5, 1)'});
        } catch (e) {}
    });
};

window.moveGameThumbPick = function(from, to) {
    var total = window.gameThumbPicked.length;
    if (from < 0 || to < 0 || from >= total || to >= total || from === to) return;
    var strip = document.getElementById('crtGameThumbPicked');
    var hadFocus = strip && strip.contains(document.activeElement);
    var flip = window.captureGameThumbFlip(strip);
    var item = window.gameThumbPicked.splice(from, 1)[0];
    window.gameThumbPicked.splice(to, 0, item);
    window.syncGameThumbInput(document.getElementById('crtGameThumbFile'));
    window.renderGameThumbPicked();
    window.playGameThumbFlip(flip);
    if (hadFocus) {
        var next = document.querySelector('#crtGameThumbPicked .crtthumbs-pick[data-index="' + (to + 1) + '"]');
        if (next && next.focus) next.focus();
    }
};

window.startGameThumbDrag = function(index, cell) {
    window.gameThumbDragIndex = index;
    if (cell) cell.classList.add('crt-dragging');
};

window.endGameThumbDrag = function() {
    window.gameThumbDragIndex = -1;
    document.querySelectorAll('.crtthumbs-pick.crt-dragging').forEach(function(c) {
        c.classList.remove('crt-dragging');
    });
    document.querySelectorAll('.crtthumbs-pick.crt-droptarget').forEach(function(c) {
        c.classList.remove('crt-droptarget');
    });
};

window.dropGameThumbPick = function(to) {
    var from = window.gameThumbDragIndex;
    window.endGameThumbDrag();
    if (from < 0 || to < 0 || from === to) return;
    window.moveGameThumbPick(from, to);
};

window.gameThumbPickCell = function(file, url, index, total) {
    var cell = document.createElement('div');
    cell.className = 'crtthumbs-pick';
    cell.draggable = true;
    cell.tabIndex = 0;
    cell.dataset.index = String(index);
    var filekey = window.gameThumbKey(file);
    cell.dataset.filekey = filekey;
    if (!window.gameThumbSeenKeys[filekey]) cell.classList.add('crt-enter');
    cell.setAttribute('aria-label', 'Thumbnail ' + index + (index === 1 ? ', main banner' : ''));
    var img = document.createElement('img');
    img.src = url;
    img.alt = file.name;
    img.title = file.name;
    img.draggable = false;
    cell.appendChild(img);
    var ord = document.createElement('span');
    ord.className = 'crtthumbs-ord';
    ord.textContent = String(index);
    ord.title = index === 1 ? 'Main banner' : 'Thumbnail ' + index;
    cell.appendChild(ord);
    var x = document.createElement('button');
    x.type = 'button';
    x.className = 'crtthumbs-x';
    x.textContent = '×';
    x.title = 'Remove this thumbnail';
    x.setAttribute('aria-label', 'Remove thumbnail ' + index);
    x.onclick = (function(idx) {
        return function() { window.removeGameThumbPick(idx); };
    })(index - 1);
    cell.appendChild(x);
    if (total > 1) {
        var left = document.createElement('button');
        left.type = 'button';
        left.className = 'crtthumbs-mv crtthumbs-mvl';
        left.innerHTML = '<i class="fa-solid fa-chevron-left"></i>';
        left.title = 'Move earlier';
        left.setAttribute('aria-label', 'Move thumbnail ' + index + ' earlier');
        if (index <= 1) left.disabled = true;
        left.onclick = (function(idx) {
            return function(ev) {
                if (ev && ev.stopPropagation) ev.stopPropagation();
                window.moveGameThumbPick(idx, idx - 1);
            };
        })(index - 1);
        cell.appendChild(left);
        var right = document.createElement('button');
        right.type = 'button';
        right.className = 'crtthumbs-mv crtthumbs-mvr';
        right.innerHTML = '<i class="fa-solid fa-chevron-right"></i>';
        right.title = 'Move later';
        right.setAttribute('aria-label', 'Move thumbnail ' + index + ' later');
        if (index >= total) right.disabled = true;
        right.onclick = (function(idx) {
            return function(ev) {
                if (ev && ev.stopPropagation) ev.stopPropagation();
                window.moveGameThumbPick(idx, idx + 1);
            };
        })(index - 1);
        cell.appendChild(right);
    }
    cell.addEventListener('dragstart', (function(idx, el) {
        return function(ev) {
            try {
                if (ev.dataTransfer) {
                    ev.dataTransfer.effectAllowed = 'move';
                    ev.dataTransfer.setData('text/plain', String(idx));
                }
            } catch (e) {}
            window.startGameThumbDrag(idx, el);
        };
    })(index - 1, cell));
    cell.addEventListener('dragover', (function(idx, el) {
        return function(ev) {
            if (window.gameThumbDragIndex < 0) return;
            if (ev.preventDefault) ev.preventDefault();
            try { if (ev.dataTransfer) ev.dataTransfer.dropEffect = 'move'; } catch (e) {}
            if (idx !== window.gameThumbDragIndex) el.classList.add('crt-droptarget');
        };
    })(index - 1, cell));
    cell.addEventListener('dragleave', (function(el) {
        return function() { el.classList.remove('crt-droptarget'); };
    })(cell));
    cell.addEventListener('drop', (function(idx) {
        return function(ev) {
            if (ev.preventDefault) ev.preventDefault();
            window.dropGameThumbPick(idx);
        };
    })(index - 1));
    cell.addEventListener('dragend', function() {
        window.endGameThumbDrag();
    });
    cell.addEventListener('keydown', (function(idx) {
        return function(ev) {
            if (!ev) return;
            if (ev.key === 'ArrowLeft') {
                if (ev.preventDefault) ev.preventDefault();
                window.moveGameThumbPick(idx, idx - 1);
            } else if (ev.key === 'ArrowRight') {
                if (ev.preventDefault) ev.preventDefault();
                window.moveGameThumbPick(idx, idx + 1);
            } else if (ev.key === 'Delete' || ev.key === 'Backspace') {
                if (ev.preventDefault) ev.preventDefault();
                window.removeGameThumbPick(idx);
            }
        };
    })(index - 1));
    return cell;
};

window.renderGameThumbPicked = function() {
    var strip = document.getElementById('crtGameThumbPicked');
    window.clearGameThumbPickURLs();
    var nameEl = document.getElementById('crtGameThumbName');
    var msgEl = document.getElementById('crtGameThumbMsg');
    if (msgEl && !msgEl.dataset.default) msgEl.dataset.default = msgEl.textContent;
    window.updateGameThumbCount();
    if (!strip) {
        window.updateGameThumbBanner();
        return;
    }
    strip.innerHTML = '';
    var present = {};
    window.gameThumbPicked.forEach(function(f) { present[window.gameThumbKey(f)] = true; });
    Object.keys(window.gameThumbSeenKeys).forEach(function(k) {
        if (!present[k]) delete window.gameThumbSeenKeys[k];
    });
    if (window.gameThumbPicked.length === 0) {
        if (nameEl) nameEl.textContent = '';
        if (msgEl && msgEl.dataset.default) msgEl.textContent = msgEl.dataset.default;
        if (document.getElementById('crtGameThumbsStrip')) {
            var empty = document.createElement('div');
            empty.className = 'feedemt';
            empty.textContent = 'New thumbnails you choose appear here before saving.';
            strip.appendChild(empty);
        }
        window.updateGameThumbBanner();
        return;
    }
    var total = window.gameThumbPicked.length;
    window.gameThumbPicked.forEach(function(f, i) {
        var url = URL.createObjectURL(f);
        window.gameThumbPickURLs.push(url);
        strip.appendChild(window.gameThumbPickCell(f, url, i + 1, total));
    });
    window.gameThumbPicked.forEach(function(f) { window.gameThumbSeenKeys[window.gameThumbKey(f)] = true; });
    if (!strip.dataset.dropBound) {
        strip.dataset.dropBound = '1';
        strip.addEventListener('dragover', function(ev) {
            if (window.gameThumbDragIndex < 0) return;
            if (ev.preventDefault) ev.preventDefault();
        });
    }
    if (nameEl) nameEl.textContent = total === 1 ? window.gameThumbPicked[0].name : total + ' images selected';
    if (msgEl) msgEl.textContent = 'Choose different files';
    window.updateGameThumbBanner();
};

window.removeGameThumbPick = function(i) {
    if (i < 0 || i >= window.gameThumbPicked.length) return;
    var file = window.gameThumbPicked[i];
    var strip = document.getElementById('crtGameThumbPicked');
    var hadFocus = strip && strip.contains(document.activeElement);
    var cell = strip ? strip.querySelector('.crtthumbs-pick[data-index="' + (i + 1) + '"]') : null;
    var finish = function() {
        var at = window.gameThumbPicked.indexOf(file);
        if (at < 0) return;
        window.gameThumbPicked.splice(at, 1);
        window.syncGameThumbInput(document.getElementById('crtGameThumbFile'));
        window.renderGameThumbPicked();
        if (hadFocus) {
            var total = window.gameThumbPicked.length;
            var pos = Math.min(at, total - 1);
            if (pos >= 0) {
                var next = document.querySelector('#crtGameThumbPicked .crtthumbs-pick[data-index="' + (pos + 1) + '"]');
                if (next && next.focus) next.focus();
            }
        }
    };
    if (cell && !window.gameThumbReducedMotion()) {
        cell.classList.add('crt-leaving');
        setTimeout(finish, 160);
    } else {
        finish();
    }
};

window.handleGameThumbFile = function(event) {
    var input = event.target;
    var fresh = window.getGameThumbFiles(input);
    if (fresh.length === 0) {
        window.syncGameThumbInput(input);
        window.renderGameThumbPicked();
        return;
    }
    if (window.gameThumbPicked.length + fresh.length > 5) {
        window.createShowError('Up to 5 thumbnails allowed.');
        window.syncGameThumbInput(input);
        window.renderGameThumbPicked();
        return;
    }
    for (var i = 0; i < fresh.length; i++) {
        if (!window.checkGameImageFile(fresh[i], 'thumbnail')) {
            window.syncGameThumbInput(input);
            window.renderGameThumbPicked();
            return;
        }
    }
    fresh.forEach(function(f) {
        var dup = window.gameThumbPicked.some(function(p) {
            return p.name === f.name && p.size === f.size && p.lastModified === f.lastModified;
        });
        if (!dup) window.gameThumbPicked.push(f);
    });
    window.syncGameThumbInput(input);
    window.renderGameThumbPicked();
};

window.gameThumbURL = function(gameId, index) {
    if (index <= 1) return '/static/uploads/games/thumbs/' + encodeURIComponent(gameId) + '.png';
    return '/static/uploads/games/thumbs/' + encodeURIComponent(gameId) + '_' + index + '.png';
};

window.gameThumbCell = function(gameId, url, index, total) {
    var cell = document.createElement('div');
    cell.className = 'crtthumbs-pick';
    var img = document.createElement('img');
    img.src = url;
    img.alt = 'Thumbnail ' + index;
    cell.appendChild(img);
    var ord = document.createElement('span');
    ord.className = 'crtthumbs-ord';
    ord.textContent = String(index);
    ord.title = index === 1 ? 'Main banner' : 'Thumbnail ' + index;
    cell.appendChild(ord);
    var x = document.createElement('button');
    x.type = 'button';
    x.className = 'crtthumbs-x';
    x.textContent = '×';
    x.title = 'Remove this thumbnail';
    x.setAttribute('aria-label', 'Remove thumbnail ' + index);
    x.onclick = function() { window.removeGameThumb(gameId, index); };
    cell.appendChild(x);
    if (total > 1) {
        var left = document.createElement('button');
        left.type = 'button';
        left.className = 'crtthumbs-mv crtthumbs-mvl';
        left.innerHTML = '<i class="fa-solid fa-chevron-left"></i>';
        left.title = 'Move earlier';
        left.setAttribute('aria-label', 'Move thumbnail ' + index + ' earlier');
        if (index <= 1) left.disabled = true;
        left.onclick = function() { window.moveGameThumb(gameId, index, index - 1); };
        cell.appendChild(left);
        var right = document.createElement('button');
        right.type = 'button';
        right.className = 'crtthumbs-mv crtthumbs-mvr';
        right.innerHTML = '<i class="fa-solid fa-chevron-right"></i>';
        right.title = 'Move later';
        right.setAttribute('aria-label', 'Move thumbnail ' + index + ' later');
        if (index >= total) right.disabled = true;
        right.onclick = function() { window.moveGameThumb(gameId, index, index + 1); };
        cell.appendChild(right);
    }
    return cell;
};

window.renderGameThumbsStrip = function(gameId, urls) {
    var bust = '?t=' + Date.now();
    var strip = document.getElementById('crtGameThumbsStrip');
    if (!urls || urls.length === 0) {
        if (strip) strip.remove();
    } else {
        if (!strip) {
            strip = document.createElement('div');
            strip.className = 'crtthumbs';
            strip.id = 'crtGameThumbsStrip';
            var thumbInput = document.getElementById('crtGameThumbFile');
            var fld = thumbInput ? thumbInput.closest('.crtfld') : null;
            var anchor = fld ? fld.querySelector('.ttpcnt') : null;
            if (!fld || !anchor) return;
            fld.insertBefore(strip, anchor);
        }
        strip.dataset.gameId = gameId;
        strip.dataset.enhanced = '1';
        strip.innerHTML = '';
        urls.forEach(function(u, i) {
            strip.appendChild(window.gameThumbCell(gameId, u + bust, i + 1, urls.length));
        });
    }
    var clearBtn = document.getElementById('crtGameThumbsClear');
    if (urls && urls.length > 0 && !clearBtn) {
        clearBtn = document.createElement('button');
        clearBtn.type = 'button';
        clearBtn.className = 'primary hpydng hpyinl hpysm';
        clearBtn.id = 'crtGameThumbsClear';
        clearBtn.innerHTML = '<span>Remove all</span>';
        clearBtn.onclick = function() { window.clearGameThumbs(gameId); };
        var prog = document.getElementById('crtGameFileProg');
        if (prog && prog.parentNode) prog.parentNode.insertBefore(clearBtn, prog);
    } else if ((!urls || urls.length === 0) && clearBtn) {
        clearBtn.remove();
    }
    var thumbPreview = document.getElementById('crtGameThumbPreview');
    if (thumbPreview) {
        window.gameThumbBannerKey = null;
        thumbPreview.src = (urls && urls.length > 0) ? urls[0] + bust : '/static/useful/temp/pfp.png';
    }
};

window.refreshGameThumbsStrip = function(gameId, count) {
    if (!count) {
        window.renderGameThumbsStrip(gameId, []);
        return;
    }
    var urls = [];
    for (var i = 1; i <= count; i++) urls.push(window.gameThumbURL(gameId, i));
    window.renderGameThumbsStrip(gameId, urls);
};

window.applyGameThumbsResult = function(gameId, out) {
    if (!out.ok) throw new Error((out.body && out.body.error) || 'Saving failed. Please try again.');
    window.renderGameThumbsStrip(gameId, (out.body && out.body.thumbs) || []);
};

window.moveGameThumb = function(gameId, from, to) {
    if (!gameId || window.gameThumbsBusy) return;
    window.gameThumbsBusy = true;
    var fd = new FormData();
    fd.append('from', String(from));
    fd.append('to', String(to));
    fetch('/create/games/' + encodeURIComponent(gameId) + '/thumbnails/move', {method: 'POST', body: fd, headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'}})
        .then(function(res) { return res.json().then(function(j) { return {ok: res.ok, body: j}; }); })
        .then(function(out) {
            window.gameThumbsBusy = false;
            window.applyGameThumbsResult(gameId, out);
        })
        .catch(function(err) {
            window.gameThumbsBusy = false;
            window.createShowError(err.message || 'Saving failed. Please try again.');
        });
};

window.removeGameThumb = function(gameId, index) {
    if (!gameId || !window.showModal || window.gameThumbsBusy) return;
    window.showModal({
        title: 'Remove thumbnail?',
        body: '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">This removes thumbnail ' + index + '. The rest shift up so the first one stays the main banner.</p>'
            + '<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:16px;">'
            + '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Cancel</span></button>'
            + '<button type="button" class="primary hpydng hpyinl hpysm" id="confirmGameThumbRemove"><span>Remove</span></button>'
            + '</div>'
    });
    setTimeout(function() {
        var cBtn = document.getElementById('confirmGameThumbRemove');
        if (cBtn) cBtn.addEventListener('click', function() {
            window.closeModal();
            window.gameThumbsBusy = true;
            var fd = new FormData();
            fd.append('index', String(index));
            fetch('/create/games/' + encodeURIComponent(gameId) + '/thumbnails/remove', {method: 'POST', body: fd, headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'}})
                .then(function(res) { return res.json().then(function(j) { return {ok: res.ok, body: j}; }); })
                .then(function(out) {
                    window.gameThumbsBusy = false;
                    window.applyGameThumbsResult(gameId, out);
                })
                .catch(function(err) {
                    window.gameThumbsBusy = false;
                    window.createShowError(err.message || 'Removing failed. Please try again.');
                });
        });
    }, 80);
};

window.initGameThumbsStrip = function() {
    var strip = document.getElementById('crtGameThumbsStrip');
    if (!strip || strip.dataset.enhanced === '1') return;
    var gameId = strip.dataset.gameId || '';
    if (!gameId) {
        var m = (window.location.pathname || '').match(/\/create\/games\/(\d+)/);
        gameId = m ? m[1] : '';
    }
    if (!gameId) return;
    var urls = [];
    strip.querySelectorAll(':scope > img').forEach(function(im) {
        var src = im.getAttribute('src');
        if (src) urls.push(src.split('?')[0]);
    });
    if (urls.length === 0) return;
    strip.dataset.enhanced = '1';
    window.renderGameThumbsStrip(gameId, urls);
};

window.clearGameThumbs = function(gameId) {
    if (!gameId || !window.showModal) return;
    window.showModal({
        title: 'Remove thumbnails?',
        body: '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">This removes all thumbnails. Your icon stays.</p>'
            + '<div style="display:flex;justify-content:flex-end;gap:8px;margin-top:16px;">'
            + '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Cancel</span></button>'
            + '<button type="button" class="primary hpydng hpyinl hpysm" id="confirmGameThumbsClear"><span>Remove</span></button>'
            + '</div>'
    });
    setTimeout(function() {
        var cBtn = document.getElementById('confirmGameThumbsClear');
        if (cBtn) cBtn.addEventListener('click', function() {
            window.closeModal();
            var nameInput = document.getElementById('crtGameName');
            var descInput = document.getElementById('crtGameDesc');
            var genreInput = document.getElementById('crtGameGenreInput');
            var tagsInput = document.getElementById('crtGameTags');
            var visInput = document.getElementById('crtGameVisibilityInput');
            var warnInput = document.getElementById('crtGameWarnings');
            var maxInput = document.getElementById('crtGameMaxPlayers');
            if (!nameInput || !nameInput.value.trim()) {
                window.createShowError('Please give your game a name.');
                return;
            }
            var fd = new FormData();
            fd.append('name', nameInput.value.trim());
            fd.append('description', descInput ? descInput.value : '');
            fd.append('genre', genreInput ? genreInput.value : '');
            fd.append('tags', tagsInput ? tagsInput.value : '');
            fd.append('visibility', visInput ? visInput.value : 'public');
            fd.append('trigger_warnings', warnInput ? warnInput.value : '');
            if (maxInput && maxInput.value.trim() !== '') fd.append('max_players', maxInput.value.trim());
            fd.append('clear_thumbnails', '1');
            fetch('/create/games/' + encodeURIComponent(gameId), {method: 'POST', body: fd, headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'}})
                .then(function(res) { return res.json().then(function(j) { return {ok: res.ok, body: j}; }); })
                .then(function(out) {
                    if (!out.ok) throw new Error((out.body && out.body.error) || 'Removing failed. Please try again.');
                    window.refreshGameThumbsStrip(gameId, 0);
                    var thumbPreview = document.getElementById('crtGameThumbPreview');
                    if (thumbPreview) {
                        window.gameThumbBannerKey = null;
                        thumbPreview.src = '/static/useful/temp/pfp.png';
                    }
                    var thumbInput = document.getElementById('crtGameThumbFile');
                    if (thumbInput) thumbInput.value = '';
                })
                .catch(function(err) {
                    window.createShowError(err.message || 'Removing failed. Please try again.');
                });
        });
    }, 80);
};

window.handleShopModelFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    var ext = (file.name.split('.').pop() || '').toLowerCase();
    if (ext !== 'obj') {
        input.value = '';
        window.createShowError('The 3D model must be an OBJ file.');
        return;
    }
    if (file.size > 25 * 1024 * 1024) {
        input.value = '';
        window.createShowError('The 3D model must be up to 25 MB.');
        return;
    }
    var nameEl = document.getElementById('crtShopModelName');
    var msgEl = document.getElementById('crtShopModelMsg');
    var viewBtn = document.getElementById('crtShopModelViewBtn');
    if (window.setCreateModelFile) window.setCreateModelFile(file);
    else window.pendingCreateModelFile = file;
    if (window.refreshCreatePreview) window.refreshCreatePreview(true);
    var transformEl = document.getElementById('crtShopModelTransform');
    if (transformEl) transformEl.value = '';
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';
    if (viewBtn) viewBtn.disabled = false;
};

window.createShowError = function(message) {
    if (!window.showModal) return;
    window.showModal({
        title: 'Hold up!',
        body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">' + crtEsc(message) + '</p>'
    });
};

window.friendlyUploadError = function(data, fallback) {
    if (data && data.error === 'rate_limited') {
        var wait = Number(data.retry_after) || 60;
        return 'Slow down. Too many uploads right now. Please try again in ' + wait + ' seconds.';
    }
    return (data && data.error) || fallback;
};

window.createShowPending = function(title, body) {
    if (!window.showModal) return;
    window.showModal({
        title: title,
        body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">' + body + '</p>'
    });
};

window.submitShopItem = function(event) {
    if (event) {
        if (event.preventDefault) event.preventDefault();
        if (event.stopPropagation) event.stopPropagation();
    }
    var form = event.target;
    var nameInput = form.querySelector('#crtShopName');
    var descInput = form.querySelector('#crtShopDesc');
    var bucksInput = form.querySelector('#crtShopPrice');
    var bitsInput = form.querySelector('#crtShopTicketsPrice');
    var fileInput = form.querySelector('#crtShopFile');
    var btn = form.querySelector('button[type="submit"]');

    var pane = document.querySelector('.pane[data-can-clothing]');
    if (pane) {
        var isClothing = (window.createCat === 'shirt' || window.createCat === 'tshirt' || window.createCat === 'pants');
        var isAccessory = (window.createCat === 'hat' || window.createCat === 'tool' || window.createCat === 'face');
        if (isClothing && pane.dataset.canClothing === 'false') {
            window.createShowError('To upload shirts, pants and t-shirts, your account must be at least 1 day old.');
            return false;
        }
        if (isAccessory && pane.dataset.canAccessories === 'false') {
            window.createShowError('To upload hats, tools and faces, your account must be at least 3 days old.');
            return false;
        }
    }

    var name = nameInput ? nameInput.value.trim() : '';
    var description = descInput ? descInput.value.trim() : '';
    var bucksRaw = bucksInput ? bucksInput.value.trim() : '';
    var bitsRaw = bitsInput ? bitsInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];

    if (!name) {
        window.createShowError('Please give your item a name.');
        return false;
    }
    if (name.length > 60) {
        window.createShowError('Item names must be 60 characters or fewer.');
        return false;
    }
    var bucks = bucksRaw === '' ? 0 : Number(bucksRaw);
    var bits = bitsRaw === '' ? 0 : Number(bitsRaw);
    if (!Number.isInteger(bucks) || bucks < 0 || bucks > 1000000) {
        window.createShowError('Please set a valid price in Vertices between 0 and 1000000.');
        return false;
    }
    if (!Number.isInteger(bits) || bits < 0 || bits > 1000000) {
        window.createShowError('Please set a valid price in Tickets between 0 and 1000000.');
        return false;
    }

    var isModelCat = (window.createCat === 'hat' || window.createCat === 'tool');
    var modelFile = window.getCreateModelFile ? window.getCreateModelFile() : null;
    if (!modelFile && window.pendingCreateModelFile) modelFile = window.pendingCreateModelFile;
    if (!modelFile) {
        var modelInput = document.getElementById('crtShopModelFile');
        if (modelInput && modelInput.files && modelInput.files[0]) modelFile = modelInput.files[0];
    }
    var checkImage = function(f, kind) {
        if (!f) {
            window.createShowError('Please choose ' + kind + '.');
            return false;
        }
        var ext = (f.name.split('.').pop() || '').toLowerCase();
        if (ext !== 'png' && ext !== 'jpg' && ext !== 'jpeg') {
            window.createShowError(kind + ' must be a PNG or JPEG image.');
            return false;
        }
        if (f.size > 8 * 1024 * 1024) {
            window.createShowError(kind + ' must be up to 8 MB.');
            return false;
        }
        return true;
    };

    if (isModelCat) {
        if (!modelFile) {
            window.createShowError('Please choose a 3D model for your item.');
            return false;
        }
        var modelExt = (modelFile.name.split('.').pop() || '').toLowerCase();
        if (modelExt !== 'obj') {
            window.createShowError('The 3D model must be an OBJ file.');
            return false;
        }
        if (modelFile.size > 25 * 1024 * 1024) {
            window.createShowError('The 3D model must be up to 25 MB.');
            return false;
        }
        if (!checkImage(file, 'a PNG or JPEG texture for your model')) return false;
    } else {
        if (!checkImage(file, 'an image file to publish')) return false;
    }

    var transformEl = document.getElementById('crtShopModelTransform');
    if (transformEl && window.getCreateModelTransformData) {
        try {
            transformEl.value = JSON.stringify(window.getCreateModelTransformData());
        } catch (e) {
            transformEl.value = '';
        }
    }

    if (btn && btn.dataset.uploading === '1') return false;

    var btnSpan = btn ? btn.querySelector('span') : null;
    var btnOrig = btn ? btn.innerHTML : '';
    var restoreBtn = function() {
        if (btn) {
            delete btn.dataset.uploading;
            btn.disabled = false;
            btn.innerHTML = btnOrig;
        }
        window.hideShopUploadProgress();
        window.crtShopUploadXhr = null;
    };
    var setBtnBusy = function(label) {
        if (btn) {
            btn.dataset.uploading = '1';
            btn.disabled = true;
        }
        if (btnSpan) btnSpan.textContent = label;
    };
    var setBtnProgress = function(pct) {
        if (!btn) return;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Publishing… ' + pct + '%';
    };

    var sendItem = function(modelToSend) {
        var fd = new FormData();
        fd.append('category', window.createCat);
        fd.append('name', name);
        fd.append('description', description);
        fd.append('bucks', String(bucks));
        fd.append('bits', String(bits));
        if (file) fd.append('file', file, file.name);
        if (isModelCat && modelToSend) fd.append('model', modelToSend, modelToSend.name || 'model.obj');
        if (transformEl && transformEl.value) fd.append('transform', transformEl.value);

        setBtnBusy('Publishing…');
        window.setShopFileProgress(0);
        if (isModelCat && modelToSend) window.setShopModelProgress(0);
        var xhr = postItem(fd, modelToSend);
        try {
            xhr.send(fd);
        } catch (e) {
            restoreBtn();
            window.createShowError('Publishing failed. Please try again.');
        }
    };

    var postItem = function(fd, model) {
        var xhr = new XMLHttpRequest();
        window.crtShopUploadXhr = xhr;
        xhr.open('POST', '/create/shop', true);
        xhr.setRequestHeader('Accept', 'application/json');
        xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
        xhr.timeout = 0;
        var fileSize = (file && file.size) || 0;
        var modelSize = (model && model.size) || 0;
        var hasModel = isModelCat && modelSize > 0;
        if (xhr.upload) {
            xhr.upload.onprogress = function(e) {
                if (e.lengthComputable) {
                    var loaded = e.loaded;
                    var filePct = fileSize > 0 ? Math.min(100, Math.round(loaded / fileSize * 100)) : 100;
                    window.setShopFileProgress(filePct);
                    if (hasModel) {
                        var modelPct = Math.min(100, Math.round(Math.max(0, loaded - fileSize) / modelSize * 100));
                        window.setShopModelProgress(modelPct);
                    }
                    var total = fileSize + modelSize;
                    setBtnProgress(total > 0 ? Math.min(100, Math.round(loaded / total * 100)) : 100);
                } else {
                    window.setShopFileProgress(100, 'Uploading…');
                    if (hasModel) window.setShopModelProgress(100, 'Uploading…');
                }
            };
        }
        xhr.onload = function() {
            var status = xhr.status || 0;
            var txt = xhr.responseText || '';
            var data = null;
            try { data = txt ? JSON.parse(txt) : null; } catch (e) { data = null; }
            if (status >= 200 && status < 300) {
                window.setShopFileProgress(100, 'Processing…');
                window.setShopModelProgress(100, 'Processing…');
                setBtnProgress(100);
                restoreBtn();
                form.reset();
                window.pendingCreateModelFile = null;
                window.pendingCreateTextureFile = null;
                if (window.setCreateModelFile) window.setCreateModelFile(null);
                if (window.setCreateTextureFile) window.setCreateTextureFile(null);
                var fileNameEl = document.getElementById('crtShopFileName');
                if (fileNameEl) fileNameEl.textContent = '';
                var modelNameEl = document.getElementById('crtShopModelName');
                if (modelNameEl) modelNameEl.textContent = '';
                if (transformEl) transformEl.value = '';
                if (window.refreshCreatePreview) window.refreshCreatePreview();
                window.updateShopPreviewName();
                if (!window.showModal) return false;
                window.showModal({
                    title: 'Item Submitted!',
                    body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Your item was submitted and is now awaiting moderation. You can check its status in My Items.</p>' +
                        '<div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">' +
                        '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Create Another</span></button>' +
                        '<button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.closeModal(); window.goToMyItems()"><span>My Items</span></button>' +
                        '</div>'
                });
                return false;
            }
            restoreBtn();
            if (status === 0) {
                window.createShowError('Connection lost. Please check your connection and try again.');
                return false;
            }
            if (status === 413) {
                window.createShowError('That file is too large for the server to accept. Try a smaller file.');
                return false;
            }
            if (!data && txt && txt.charAt(0) === '<') {
                if (status === 401 || status === 403) {
                    window.createShowError('Your session expired. Please log in again, then retry.');
                    return false;
                }
                if (status === 502 || status === 503 || status === 504) {
                    window.createShowError('The server took too long to respond. Please try again, and try a smaller file if it keeps happening.');
                    return false;
                }
                window.createShowError('Publishing failed. Please try again.');
                return false;
            }
            window.createShowError(window.friendlyUploadError(data, 'Publishing failed. Please try again.'));
            return false;
        };
        xhr.onerror = function() {
            restoreBtn();
            window.createShowError('Connection lost. Please check your connection and try again.');
        };
        xhr.ontimeout = function() {
            restoreBtn();
            window.createShowError('The upload timed out. On a slow connection, try a smaller file and try again.');
        };
        xhr.onabort = function() {
            restoreBtn();
        };
        return xhr;
    };

    if (isModelCat && window.getCreateTransformedModel) {
        setBtnBusy('Preparing…');
        window.setShopModelProgress(0, 'Preparing…');
        window.getCreateTransformedModel().then(function(baked) {
            sendItem(baked || modelFile);
        }, function() {
            sendItem(modelFile);
        });
    } else {
        sendItem(modelFile);
    }
    return false;
};

window.setShopFileProgress = function(percent, label) {
    var wrap = document.getElementById('crtShopFileProg');
    var bar = document.getElementById('crtShopFileProgBar');
    var txt = document.getElementById('crtShopFileProgTxt');
    if (!wrap || !bar || !txt) return;
    var pct = Math.max(0, Math.min(100, Math.round(percent)));
    wrap.hidden = false;
    bar.style.width = pct + '%';
    txt.textContent = label || ('Uploading… ' + pct + '%');
};

window.setShopModelProgress = function(percent, label) {
    var wrap = document.getElementById('crtShopModelProg');
    var bar = document.getElementById('crtShopModelProgBar');
    var txt = document.getElementById('crtShopModelProgTxt');
    if (!wrap || !bar || !txt) return;
    var pct = Math.max(0, Math.min(100, Math.round(percent)));
    wrap.hidden = false;
    bar.style.width = pct + '%';
    txt.textContent = label || ('Uploading… ' + pct + '%');
};

window.hideShopUploadProgress = function() {
    ['crtShopFileProg', 'crtShopModelProg'].forEach(function(id) {
        var wrap = document.getElementById(id);
        if (wrap) wrap.hidden = true;
    });
    ['crtShopFileProgBar', 'crtShopModelProgBar'].forEach(function(id) {
        var bar = document.getElementById(id);
        if (bar) bar.style.width = '0%';
    });
};

window.goToMyItems = function() {
    try {
        if (window.navigateWithHistory) {
            window.navigateWithHistory('/create/items');
            return;
        }
        if (window.htmx) {
            window.htmx.ajax('GET', '/create/items', { target: 'body' });
            return;
        }
    } catch (e) {}
    window.location.href = '/create/items';
};

window.setGameFileProgress = function(percent, label) {
    var wrap = document.getElementById('crtGameFileProg');
    var bar = document.getElementById('crtGameFileProgBar');
    var txt = document.getElementById('crtGameFileProgTxt');
    if (!wrap || !bar || !txt) return;
    var pct = Math.max(0, Math.min(100, Math.round(percent)));
    wrap.hidden = false;
    bar.style.width = pct + '%';
    txt.textContent = label || ('Uploading… ' + pct + '%');
};

window.hideGameUploadProgress = function() {
    var wrap = document.getElementById('crtGameFileProg');
    var bar = document.getElementById('crtGameFileProgBar');
    if (wrap) wrap.hidden = true;
    if (bar) bar.style.width = '0%';
};

window.resetGameDropdowns = function() {
    var genreInput = document.getElementById('crtGameGenreInput');
    var genreLabel = document.getElementById('crtGameGenreLabel');
    var genreDropdown = document.getElementById('crtGameGenreDropdown');
    if (genreInput) genreInput.value = '';
    if (genreLabel) genreLabel.textContent = 'Choose a genre';
    if (genreDropdown) genreDropdown.querySelectorAll('.glb-dropdown-item').forEach(function(item) {
        item.classList.remove('active');
    });
    var visInput = document.getElementById('crtGameVisibilityInput');
    var visLabel = document.getElementById('crtGameVisibilityLabel');
    var visDropdown = document.getElementById('crtGameVisibilityDropdown');
    if (visInput) visInput.value = 'public';
    if (visLabel) visLabel.textContent = 'Public';
    if (visDropdown) visDropdown.querySelectorAll('.glb-dropdown-item').forEach(function(item) {
        item.classList.toggle('active', item.dataset.value === 'public');
    });
};

window.goToGame = function(id) {
    var url = '/play/' + encodeURIComponent(id);
    try {
        if (window.navigateWithHistory) {
            window.navigateWithHistory(url);
            return;
        }
        if (window.htmx) {
            window.htmx.ajax('GET', url, { target: 'body' });
            return;
        }
    } catch (e) {}
    window.location.href = url;
};

window.submitGame = function(event) {
    if (event) {
        if (event.preventDefault) event.preventDefault();
        if (event.stopPropagation) event.stopPropagation();
    }
    var form = event.target;
    if (form && form.tagName !== 'FORM') {
        form = form.closest ? form.closest('form') : document.querySelector('#crt-game form');
    }
    var nameInput = form.querySelector('#crtGameName');
    var descInput = form.querySelector('#crtGameDesc');
    var genreInput = form.querySelector('#crtGameGenreInput');
    var tagsInput = form.querySelector('#crtGameTags');
    var visInput = form.querySelector('#crtGameVisibilityInput');
    var fileInput = form.querySelector('#crtGameFile');
    var btn = form.querySelector('button[type="submit"]');

    var pane = document.querySelector('.pane[data-can-public-game], .pane[data-can-game]');
    var visibility = visInput ? visInput.value : 'public';
    var canPublic = pane ? pane.dataset.canPublicGame : undefined;
    if (canPublic === undefined && pane) canPublic = pane.dataset.canGame === 'false' ? 'false' : 'true';
    if (visibility !== 'private' && canPublic === 'false') {
        window.createShowError('To create a public game, your account must be at least 1 day old. You can still create a private game.');
        return false;
    }

    var thumbInput = form.querySelector('#crtGameThumbFile');
    var name = nameInput ? nameInput.value.trim() : '';
    var description = descInput ? descInput.value.trim() : '';
    var genre = genreInput ? genreInput.value : '';
    var tags = tagsInput ? tagsInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];
    var thumbs = window.getGameThumbFiles(thumbInput);

    if (!name) {
        window.createShowError('Please give your game a name.');
        return false;
    }
    if (name.length > 60) {
        window.createShowError('Game names must be 60 characters or fewer.');
        return false;
    }
    if (description.length > 1000) {
        window.createShowError('Game descriptions must be 1000 characters or fewer.');
        return false;
    }
    if (tags.length > 255) {
        window.createShowError('Game tags must be 255 characters or fewer.');
        return false;
    }
    if (file && !window.checkGameImageFile(file, 'icon')) return false;
    if (!window.checkGameThumbFiles(thumbs)) return false;

    if (btn && btn.dataset.uploading === '1') return false;

    var fd = new FormData();
    fd.append('name', name);
    fd.append('description', description);
    fd.append('genre', genre);
    fd.append('tags', tags);
    fd.append('visibility', visibility);
    if (file) fd.append('icon', file, file.name);
    thumbs.forEach(function(t) { fd.append('thumbnail', t, t.name); });

    var btnOriginal = btn ? btn.innerHTML : '';
    var restoreBtn = function() {
        if (btn) {
            delete btn.dataset.uploading;
            btn.disabled = false;
            btn.innerHTML = btnOriginal;
        }
        window.hideGameUploadProgress();
        window.crtGameUploadXhr = null;
    };
    var setBtnProgress = function(pct) {
        if (!btn) return;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Publishing… ' + pct + '%';
    };

    if (btn) {
        btn.dataset.uploading = '1';
        btn.disabled = true;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Publishing…';
    }
    window.setGameFileProgress(0);

    var xhr = new XMLHttpRequest();
    window.crtGameUploadXhr = xhr;
    xhr.open('POST', '/create/game', true);
    xhr.setRequestHeader('Accept', 'application/json');
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
    xhr.timeout = 0;
    if (xhr.upload) {
        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                var pct = Math.round((e.loaded / e.total) * 100);
                window.setGameFileProgress(pct);
                setBtnProgress(pct);
            } else {
                window.setGameFileProgress(100, 'Uploading…');
            }
        };
    }
    xhr.onload = function() {
        var status = xhr.status || 0;
        var txt = xhr.responseText || '';
        var data = null;
        try { data = txt ? JSON.parse(txt) : null; } catch (e) { data = null; }
        if (status >= 200 && status < 300 && data && data.id) {
            window.setGameFileProgress(100, 'Processing…');
            setBtnProgress(100);
            var gameId = data.id;
            restoreBtn();
            form.reset();
            window.resetGameDropdowns();
            window.updateGamePreviewName();
            var fileNameEl = document.getElementById('crtGameFileName');
            if (fileNameEl) fileNameEl.textContent = '';
            var fileMsgEl = document.getElementById('crtGameFileMsg');
            if (fileMsgEl) fileMsgEl.textContent = 'Choose a PNG or JPEG';
            window.gameThumbPicked = [];
            window.renderGameThumbPicked();
            var thumbMsgEl = document.getElementById('crtGameThumbMsg');
            if (thumbMsgEl) thumbMsgEl.textContent = 'Choose PNG or JPEG images';
            var previewImg = document.getElementById('crtGamePreviewImg');
            if (previewImg) previewImg.src = '/static/useful/temp/pfp.png';
            if (!window.showModal) {
                window.goToGame(gameId);
                return false;
            }
            window.showModal({
                title: 'Game Created!',
                body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Your game is live and players can discover it on the Play page.</p>' +
                    '<div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">' +
                    '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Create Another</span></button>' +
                    '<button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.closeModal(); window.goToGame(' + gameId + ')"><span>View Game</span></button>' +
                    '</div>'
            });
            return false;
        }
        restoreBtn();
        if (status === 0) {
            window.createShowError('Connection lost. Please check your connection and try again.');
            return false;
        }
        if (status === 413) {
            window.createShowError('That file is too large for the server to accept. Try a smaller file.');
            return false;
        }
        if (!data && txt && txt.charAt(0) === '<') {
            if (status === 401 || status === 403) {
                window.createShowError('Your session expired. Please log in again, then retry.');
                return false;
            }
            if (status === 502 || status === 503 || status === 504) {
                window.createShowError('The server took too long to respond. Please try again, and try a smaller file if it keeps happening.');
                return false;
            }
            window.createShowError('Publishing failed. Please try again.');
            return false;
        }
        window.createShowError(window.friendlyUploadError(data, 'Publishing failed. Please try again.'));
        return false;
    };
    xhr.onerror = function() {
        restoreBtn();
        window.createShowError('Connection lost. Please check your connection and try again.');
    };
    xhr.ontimeout = function() {
        restoreBtn();
        window.createShowError('The upload timed out. On a slow connection, try a smaller file and try again.');
    };
    xhr.onabort = function() {
        restoreBtn();
    };
    try {
        xhr.send(fd);
    } catch (e) {
        restoreBtn();
        window.createShowError('Publishing failed. Please try again.');
    }
    return false;
};

window.submitGameEdit = function(event, gameId) {
    if (event) {
        if (event.preventDefault) event.preventDefault();
        if (event.stopPropagation) event.stopPropagation();
    }
    if (!gameId) return false;
    var form = event.target;
    if (form && form.tagName !== 'FORM') {
        form = form.closest ? form.closest('form') : document.querySelector('form');
    }
    var nameInput = form.querySelector('#crtGameName');
    var descInput = form.querySelector('#crtGameDesc');
    var genreInput = form.querySelector('#crtGameGenreInput');
    var tagsInput = form.querySelector('#crtGameTags');
    var visInput = form.querySelector('#crtGameVisibilityInput');
    var warnInput = form.querySelector('#crtGameWarnings');
    var fileInput = form.querySelector('#crtGameFile');
    var thumbInput = form.querySelector('#crtGameThumbFile');
    var btn = form.querySelector('button[type="submit"]');

    var name = nameInput ? nameInput.value.trim() : '';
    var description = descInput ? descInput.value.trim() : '';
    var genre = genreInput ? genreInput.value : '';
    var tags = tagsInput ? tagsInput.value.trim() : '';
    var visibility = visInput ? visInput.value : 'public';
    var triggerWarnings = warnInput ? warnInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];
    var thumbs = window.getGameThumbFiles(thumbInput);

    if (!name) {
        window.createShowError('Please give your game a name.');
        return false;
    }
    if (name.length > 60) {
        window.createShowError('Game names must be 60 characters or fewer.');
        return false;
    }
    if (description.length > 1000) {
        window.createShowError('Game descriptions must be 1000 characters or fewer.');
        return false;
    }
    if (tags.length > 255) {
        window.createShowError('Game tags must be 255 characters or fewer.');
        return false;
    }
    if (triggerWarnings.length > 500) {
        window.createShowError('Game trigger warnings must be 500 characters or fewer.');
        return false;
    }
    var maxEditInput = form.querySelector('#crtGameMaxPlayers');
    var maxEditPlayers = maxEditInput ? maxEditInput.value.trim() : '';
    if (maxEditPlayers !== '') {
        var maxEditNum = Number(maxEditPlayers);
        if (!Number.isInteger(maxEditNum) || maxEditNum < 2 || maxEditNum > 100) {
            window.createShowError('Max players must be between 2 and 100.');
            return false;
        }
    }
    if (file && !window.checkGameImageFile(file, 'icon')) return false;
    if (!window.checkGameThumbFiles(thumbs)) return false;

    if (btn && btn.dataset.uploading === '1') return false;

    var fd = new FormData();
    fd.append('name', name);
    fd.append('description', description);
    fd.append('genre', genre);
    fd.append('tags', tags);
    fd.append('visibility', visibility);
    fd.append('trigger_warnings', triggerWarnings);
    if (maxEditPlayers !== '') fd.append('max_players', maxEditPlayers);
    if (file) fd.append('icon', file, file.name);
    thumbs.forEach(function(t) { fd.append('thumbnail', t, t.name); });

    var btnOriginal = btn ? btn.innerHTML : '';
    var restoreBtn = function() {
        if (btn) {
            delete btn.dataset.uploading;
            btn.disabled = false;
            btn.innerHTML = btnOriginal;
        }
        window.hideGameUploadProgress();
        window.crtGameUploadXhr = null;
    };
    var setBtnProgress = function(pct) {
        if (!btn) return;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Saving… ' + pct + '%';
    };

    if (btn) {
        btn.dataset.uploading = '1';
        btn.disabled = true;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Saving…';
    }
    window.setGameFileProgress(0);

    var xhr = new XMLHttpRequest();
    window.crtGameUploadXhr = xhr;
    xhr.open('POST', '/create/games/' + encodeURIComponent(gameId), true);
    xhr.setRequestHeader('Accept', 'application/json');
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
    xhr.timeout = 0;
    if (xhr.upload) {
        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                var pct = Math.round((e.loaded / e.total) * 100);
                window.setGameFileProgress(pct);
                setBtnProgress(pct);
            } else {
                window.setGameFileProgress(100, 'Uploading…');
            }
        };
    }
    xhr.onload = function() {
        var status = xhr.status || 0;
        var txt = xhr.responseText || '';
        var data = null;
        try { data = txt ? JSON.parse(txt) : null; } catch (e) { data = null; }
        if (status >= 200 && status < 300 && data && data.success) {
            window.setGameFileProgress(100, 'Processing…');
            setBtnProgress(100);
            restoreBtn();
            var bust = '?t=' + Date.now();
            var previewImg = document.getElementById('crtGamePreviewImg');
            if (previewImg) previewImg.src = '/static/uploads/games/' + encodeURIComponent(gameId) + '.png' + bust;
            var thumbPreview = document.getElementById('crtGameThumbPreview');
            if (thumbPreview) {
                window.gameThumbBannerKey = null;
                thumbPreview.src = '/static/uploads/games/thumbs/' + encodeURIComponent(gameId) + '.png' + bust;
            }
            var pickedCount = window.gameThumbPicked.length;
            window.gameThumbPicked = [];
            var thumbInput = document.getElementById('crtGameThumbFile');
            if (thumbInput) thumbInput.value = '';
            window.renderGameThumbPicked();
            var thumbMsgEl = document.getElementById('crtGameThumbMsg');
            if (thumbMsgEl) thumbMsgEl.textContent = 'Choose new thumbnails';
            if (pickedCount > 0) window.refreshGameThumbsStrip(gameId, pickedCount);
            window.updateGamePreviewName();
            if (!window.showModal) return false;
            window.showModal({
                title: 'Game Updated!',
                body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Your changes are live.</p>' +
                    '<div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">' +
                    '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Keep Editing</span></button>' +
                    '<button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.closeModal(); window.goToGame(' + gameId + ')"><span>View Game</span></button>' +
                    '</div>'
            });
            return false;
        }
        restoreBtn();
        if (status === 0) {
            window.createShowError('Connection lost. Please check your connection and try again.');
            return false;
        }
        if (status === 413) {
            window.createShowError('That file is too large for the server to accept. Try a smaller file.');
            return false;
        }
        if (!data && txt && txt.charAt(0) === '<') {
            if (status === 401 || status === 403) {
                window.createShowError('Your session expired. Please log in again, then retry.');
                return false;
            }
            if (status === 502 || status === 503 || status === 504) {
                window.createShowError('The server took too long to respond. Please try again, and try a smaller file if it keeps happening.');
                return false;
            }
            window.createShowError('Saving failed. Please try again.');
            return false;
        }
        window.createShowError(window.friendlyUploadError(data, 'Saving failed. Please try again.'));
        return false;
    };
    xhr.onerror = function() {
        restoreBtn();
        window.createShowError('Connection lost. Please check your connection and try again.');
    };
    xhr.ontimeout = function() {
        restoreBtn();
        window.createShowError('The upload timed out. On a slow connection, try a smaller file and try again.');
    };
    xhr.onabort = function() {
        restoreBtn();
    };
    try {
        xhr.send(fd);
    } catch (e) {
        restoreBtn();
        window.createShowError('Saving failed. Please try again.');
    }
    return false;
};

window.setAssetUploadProgress = function(percent, label) {
    var wrap = document.getElementById('crtAssetProg');
    var bar = document.getElementById('crtAssetProgBar');
    var txt = document.getElementById('crtAssetProgTxt');
    if (!wrap || !bar || !txt) return;
    var pct = Math.max(0, Math.min(100, Math.round(percent)));
    wrap.hidden = false;
    bar.style.width = pct + '%';
    txt.textContent = label || ('Uploading… ' + pct + '%');
};

window.hideAssetUploadProgress = function() {
    var wrap = document.getElementById('crtAssetProg');
    var bar = document.getElementById('crtAssetProgBar');
    if (wrap) wrap.hidden = true;
    if (bar) bar.style.width = '0%';
};

window.submitAsset = function(event) {
    if (event) {
        if (event.preventDefault) event.preventDefault();
        if (event.stopPropagation) event.stopPropagation();
    }
    var form = event.target;
    if (form && form.tagName !== 'FORM') {
        form = form.closest ? form.closest('form') : document.querySelector('#crt-asset form');
    }
    var nameInput = form.querySelector('#crtAssetName');
    var fileInput = form.querySelector('#crtAssetFile');
    var name = nameInput ? nameInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];

    var pane = document.querySelector('.pane[data-can-image]');
    if (pane) {
        if (window.assetCategory === 'image' && pane.dataset.canImage === 'false') {
            window.createShowError('To upload an image asset, your account must be at least 3 days old.');
            return false;
        }
        if ((window.assetCategory === 'mesh' || window.assetCategory === 'sound') && pane.dataset.canMeshSound === 'false') {
            window.createShowError('To upload a mesh or a sound, your account must be at least 7 days old.');
            return false;
        }
    }

    if (!name) {
        window.createShowError('Please give your asset a name.');
        return false;
    }
    if (!file) {
        window.createShowError('Please choose a file to upload.');
        return false;
    }

    var info = ASSET_TYPE_INFO[window.assetCategory] || ASSET_TYPE_INFO.image;
    var ext = (file.name.split('.').pop() || '').toLowerCase();

    if (info.exts.indexOf(ext) === -1) {
        window.createShowError('That file type is not supported for ' + info.name + ' assets. Supported formats: ' + info.formats + '.');
        return false;
    }
    if (file.size > info.max) {
        window.createShowError(info.name + ' assets must be ' + info.limit.toLowerCase() + '.');
        return false;
    }

    var btn = form.querySelector('button[type="submit"]');
    if (btn && btn.dataset.uploading === '1') return false;

    var fd = new FormData(form);
    fd.set('type', window.assetCategory);

    var btnOriginal = btn ? btn.innerHTML : '';
    if (btn) {
        btn.dataset.uploading = '1';
        btn.disabled = true;
        btn.innerHTML = '<svg class="dmspn" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">' +
            '<rect class="sq sq-1" x="43" y="7" width="14" height="14"/>' +
            '<rect class="sq sq-2" x="25" y="25" width="14" height="14"/>' +
            '<rect class="sq sq-3" x="7" y="43" width="14" height="14"/>' +
            '<rect class="sq sq-4" x="25" y="61" width="14" height="14"/>' +
            '<rect class="sq sq-5" x="43" y="79" width="14" height="14"/>' +
            '<rect class="sq sq-6" x="61" y="61" width="14" height="14"/>' +
            '<rect class="sq sq-7" x="79" y="43" width="14" height="14"/>' +
            '<rect class="sq sq-8" x="61" y="25" width="14" height="14"/>' +
            '</svg><span>Uploading… 0%</span>';
    }
    window.setAssetUploadProgress(0);
    var restoreBtn = function() {
        if (btn) {
            delete btn.dataset.uploading;
            btn.disabled = false;
            btn.innerHTML = btnOriginal;
        }
        window.hideAssetUploadProgress();
        window.crtAssetUploadXhr = null;
    };
    var resetAssetForm = function() {
        form.reset();
        window.updateAssetTab();
        window.updateAssetPreviewName();
    };
    var setBtnProgress = function(pct) {
        if (!btn) return;
        var span = btn.querySelector('span');
        if (span) span.textContent = 'Uploading… ' + pct + '%';
    };

    var xhr = new XMLHttpRequest();
    window.crtAssetUploadXhr = xhr;
    xhr.open('POST', '/create/assets', true);
    xhr.setRequestHeader('Accept', 'application/json');
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
    xhr.timeout = 0;
    if (xhr.upload) {
        xhr.upload.onprogress = function(e) {
            if (e.lengthComputable) {
                var pct = Math.round((e.loaded / e.total) * 100);
                window.setAssetUploadProgress(pct);
                setBtnProgress(pct);
            } else {
                window.setAssetUploadProgress(100, 'Uploading…');
            }
        };
    }
    xhr.onload = function() {
        var status = xhr.status || 0;
        var txt = xhr.responseText || '';
        var data = null;
        try { data = txt ? JSON.parse(txt) : null; } catch (e) { data = null; }
        if (status >= 200 && status < 300) {
            window.setAssetUploadProgress(100, 'Processing…');
            setBtnProgress(100);
            restoreBtn();
            resetAssetForm();
            if (!window.showModal) return;
            window.showModal({
                title: 'Asset Uploaded!',
                body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">Your asset was uploaded and is now awaiting moderation. You can preview it in My Assets.</p>' +
                    '<div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px;">' +
                    '<button type="button" class="primary hpyinf hpyinl hpysm" onclick="window.closeModal()"><span>Upload Another</span></button>' +
                    '<button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.closeModal(); window.goToMyAssets()"><span>My Assets</span></button>' +
                    '</div>'
            });
            return;
        }
        restoreBtn();
        if (status === 0) {
            window.createShowError('Connection lost during upload. Please check your connection and try again.');
            return;
        }
        if (status === 413) {
            window.createShowError('That file is too large for the server to accept. Try a smaller file.');
            return;
        }
        if (!data && txt && txt.charAt(0) === '<') {
            if (status === 401 || status === 403) {
                window.createShowError('Your session expired. Please log in again, then retry the upload.');
                return;
            }
            if (status === 502 || status === 503 || status === 504) {
                window.createShowError('The server took too long to respond. Please try again, and try a smaller file if it keeps happening.');
                return;
            }
            window.createShowError('Upload failed. Please try again.');
            return;
        }
        window.createShowError(window.friendlyUploadError(data, 'Upload failed. Please try again.'));
    };
    xhr.onerror = function() {
        restoreBtn();
        window.createShowError('Connection lost during upload. Please check your connection and try again.');
    };
    xhr.ontimeout = function() {
        restoreBtn();
        window.createShowError('The upload timed out. On a slow connection, try a smaller file and try again.');
    };
    xhr.onabort = function() {
        restoreBtn();
    };
    try {
        xhr.send(fd);
    } catch (e) {
        restoreBtn();
        window.createShowError('Upload failed. Please try again.');
    }

    return false;
};

window.goToMyAssets = function() {
    try {
        if (window.navigateWithHistory) {
            window.navigateWithHistory('/create/assets');
            return;
        }
        if (window.htmx) {
            window.htmx.ajax('GET', '/create/assets', { target: 'body' });
            return;
        }
    } catch (e) {}
    window.location.href = '/create/assets';
};

window.updateCreateUrl = function() {
    if (!document.querySelector('.crttypebtn')) return;
    try {
        var url = new URL(window.location.href);
        url.searchParams.set('type', window.createType);
        if (window.createType === 'shop') {
            url.searchParams.set('cat', window.createCat);
        } else {
            url.searchParams.delete('cat');
        }
        if (window.createType === 'asset') {
            url.searchParams.set('asset', window.assetCategory);
        } else {
            url.searchParams.delete('asset');
        }
        var search = url.searchParams.toString();
        window.history.replaceState({}, document.title, search ? url.pathname + '?' + search : url.pathname);
    } catch (e) {}
};

window.initCreatePage = function() {
    var activeTypeTab = document.querySelector('.crttypebtn.active');
    if (activeTypeTab && activeTypeTab.dataset.type) {
        window.createType = activeTypeTab.dataset.type;
    }
    var activeCatTab = document.querySelector('.crtshoptabs .tabbtn.active');
    if (activeCatTab && activeCatTab.dataset.cat) {
        window.createCat = activeCatTab.dataset.cat;
    }
    var activeAssetTab = document.querySelector('.crtassettabs .tabbtn.active');
    if (activeAssetTab && activeAssetTab.dataset.assettype) {
        window.assetCategory = activeAssetTab.dataset.assettype;
    }
    if (window.setCreateCategory) window.setCreateCategory(window.createCat);

    var previewImg = document.getElementById('crtShopPreviewImg');
    if (previewImg && previewImg.complete && previewImg.naturalWidth > 0) {
        previewImg.classList.add('ld');
    } else if (previewImg) {
        previewImg.addEventListener('load', function() {
            previewImg.classList.add('ld');
        });
    }

    window.updateShopCategoryIndicator();
    window.updateAssetCategoryIndicator();
    if (window.crtAssetUploadXhr || window.crtShopUploadXhr || window.crtGameUploadXhr) return;
    window.updateAssetTab();
    window.updateCreateUrl();

    try {
        var errMsg = new URL(window.location.href).searchParams.get('error');
        if (errMsg) {
            window.createShowError(errMsg);
            var cleanUrl = new URL(window.location.href);
            cleanUrl.searchParams.delete('error');
            var search = cleanUrl.searchParams.toString();
            window.history.replaceState({}, document.title, search ? cleanUrl.pathname + '?' + search : cleanUrl.pathname);
        }
    } catch (e) {}

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateAssetCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 50);

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateAssetCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 150);

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateAssetCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 350);
};

window.initCreatePage();

window.initGameThumbsStrip();

window.initGameThumbPicked = function() {
    var strip = document.getElementById('crtGameThumbPicked');
    if (!strip || strip.children.length > 0) return;
    window.renderGameThumbPicked();
};

window.initGameThumbPicked();

if (!window.__gameThumbsStripBound) {
    window.__gameThumbsStripBound = true;
    document.addEventListener('htmx:afterSettle', function() {
        window.initGameThumbsStrip();
        window.initGameThumbPicked();
    });
}

document.addEventListener('htmx:afterSettle', function() {
    if (document.getElementById('crt-asset') || document.getElementById('crt-shop')) {
        window.initCreatePage();
    }
});

window.addEventListener('beforeunload', function(e) {
    if (window.crtAssetUploadXhr || window.crtShopUploadXhr || window.crtGameUploadXhr) {
        e.preventDefault();
        e.returnValue = '';
    }
});

window.crtBlockHtmxDuringUpload = function(e) {
    if (window.crtAssetUploadXhr || window.crtShopUploadXhr || window.crtGameUploadXhr) {
        if (e && e.preventDefault) e.preventDefault();
        if (e && e.stopPropagation) e.stopPropagation();
        window.createShowError('Please wait for the upload to finish before leaving this page.');
        return false;
    }
};

document.addEventListener('htmx:confirm', window.crtBlockHtmxDuringUpload);
document.addEventListener('htmx:beforeRequest', window.crtBlockHtmxDuringUpload);

window.addEventListener('resize', function() {
    window.updateShopCategoryIndicator();
    window.updateAssetCategoryIndicator();
});
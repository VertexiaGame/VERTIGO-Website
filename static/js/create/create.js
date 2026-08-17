window.createType = 'shop';
window.createCat = 'hat';

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
    var active = document.querySelector('.crtvwp .settabcntnt.active');
    if (!viewport || !active) return;
    viewport.style.height = active.offsetHeight + 'px';
    if (window.createHeightTimeout) clearTimeout(window.createHeightTimeout);
    window.createHeightTimeout = setTimeout(function() {
        viewport.style.height = '';
    }, 350);
};

window.switchCreateType = function(type, event) {
    var tab = event ? event.currentTarget : null;
    if (!tab || tab.classList.contains('active')) return;

    var current = document.querySelector('.crtvwp .settabcntnt.active');
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

    target.querySelectorAll('.setcrd').forEach(function(card) {
        card.style.animation = 'none';
        void card.offsetWidth;
        card.style.animation = '';
    });

    window.createType = type;
    window.updateCreateViewportHeight();
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
    if (window.setCreateCategory) window.setCreateCategory(cat);
    if (window.refreshCreatePreview) window.refreshCreatePreview();
    var display = CRT_CAT_NAMES[cat] || cat;

    var modelField = document.getElementById('crtShopModelFld');
    if (modelField) {
        modelField.classList.toggle('visible', cat === 'hat' || cat === 'face' || cat === 'tool');
    }

    var titleEl = document.getElementById('crtShopTitle');
    if (titleEl) titleEl.textContent = 'Create ' + display;
    var catNameEl = document.getElementById('crtShopCatName');
    if (catNameEl) catNameEl.textContent = display;
    var input = document.getElementById('crtShopCatInput');
    if (input) input.value = cat;

    window.updateShopPreviewName();
    window.updateShopCategoryIndicator();
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
    if (window.refreshCreatePreview) window.refreshCreatePreview();
};

window.handleGameFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    var nameEl = document.getElementById('crtGameFileName');
    var msgEl = document.getElementById('crtGameFileMsg');
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';
};

window.handleAssetFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    var nameEl = document.getElementById('crtAssetFileName');
    var msgEl = document.getElementById('crtAssetFileMsg');
    if (nameEl) nameEl.textContent = file.name;
    if (msgEl) msgEl.textContent = 'Choose a different file';
};

window.handleShopModelFile = function(event) {
    var input = event.target;
    var file = input.files && input.files[0];
    if (!file) return;
    var ext = (file.name.split('.').pop() || '').toLowerCase();
    if (ext !== 'glb' && ext !== 'obj') {
        input.value = '';
        window.createShowError('The 3D model must be a GLB or OBJ file.');
        return;
    }
    var nameEl = document.getElementById('crtShopModelName');
    var msgEl = document.getElementById('crtShopModelMsg');
    var viewBtn = document.getElementById('crtShopModelViewBtn');
    window.setCreateModelFile(file);
    if (window.refreshCreatePreview) window.refreshCreatePreview();
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

window.createShowPending = function(title, body) {
    if (!window.showModal) return;
    window.showModal({
        title: title,
        body: '<p style="font-family: \'Ubuntu\', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">' + body + '</p>'
    });
};
window.submitShopItem = function(event) {
    event.preventDefault();
    var form = event.target;
    var nameInput = form.querySelector('#crtShopName');
    var priceInput = form.querySelector('#crtShopPrice');
    var fileInput = form.querySelector('#crtShopFile');

    var name = nameInput ? nameInput.value.trim() : '';
    var price = priceInput ? priceInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];

    if (!name) {
        window.createShowError('Please give your item a name.');
        return false;
    }
    if (price === '' || isNaN(Number(price)) || Number(price) < 0) {
        window.createShowError('Please set a valid price in Vertices.');
        return false;
    }

    if (window.createCat === 'hat' || window.createCat === 'face' || window.createCat === 'tool') {
        var modelFile = window.getCreateModelFile ? window.getCreateModelFile() : null;
        if (!modelFile) {
            window.createShowError('Please choose a 3D model for your item.');
            return false;
        }
        var modelExt = (modelFile.name.split('.').pop() || '').toLowerCase();
        if (modelExt === 'obj' && !file) {
            window.createShowError('OBJ models require an asset texture file.');
            return false;
        }
    } else {
        if (!file) {
            window.createShowError('Please choose an asset file to publish.');
            return false;
        }
    }

    var catName = CRT_CAT_NAMES[window.createCat] || window.createCat;
    var safeName = crtEsc(name);
    var transformEl = document.getElementById('crtShopModelTransform');
    if (transformEl && window.getCreateModelTransformData) {
        transformEl.value = JSON.stringify(window.getCreateModelTransformData());
    }
    window.createShowPending('Publish ' + catName,
        'Publishing <strong>' + safeName + '</strong> to the shop will open once item creation is enabled on the server.');
    return false;
};

window.submitGame = function(event) {
    event.preventDefault();
    var form = event.target;
    var nameInput = form.querySelector('#crtGameName');
    var name = nameInput ? nameInput.value.trim() : '';
    if (!name) {
        window.createShowError('Please give your game a name.');
        return false;
    }
    window.createShowPending('Create Game',
        'Game creation will open here once game publishing is enabled on the server.');
    return false;
};

window.submitAsset = function(event) {
    event.preventDefault();
    var form = event.target;
    var nameInput = form.querySelector('#crtAssetName');
    var fileInput = form.querySelector('#crtAssetFile');
    var name = nameInput ? nameInput.value.trim() : '';
    var file = fileInput && fileInput.files && fileInput.files[0];
    if (!name) {
        window.createShowError('Please give your asset a name.');
        return false;
    }
    if (!file) {
        window.createShowError('Please choose a file to upload.');
        return false;
    }
    window.createShowPending('Upload Asset',
        'Asset uploads will open here once asset publishing is enabled on the server.');
    return false;
};

window.updateCreateUrl = function() {
    try {
        var url = new URL(window.location.href);
        url.searchParams.set('type', window.createType);
        if (window.createType === 'shop') {
            url.searchParams.set('cat', window.createCat);
        } else {
            url.searchParams.delete('cat');
        }
        var search = url.searchParams.toString();
        window.history.replaceState({}, document.title, search ? url.pathname + '?' + search : url.pathname);
    } catch (e) {}
};

(function() {
    var activeTypeTab = document.querySelector('.crttypebtn.active');
    if (activeTypeTab && activeTypeTab.dataset.type) {
        window.createType = activeTypeTab.dataset.type;
    }
    var activeCatTab = document.querySelector('.crtshoptabs .tabbtn.active');
    if (activeCatTab && activeCatTab.dataset.cat) {
        window.createCat = activeCatTab.dataset.cat;
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
    window.updateCreateUrl();

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 50);

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 150);

    setTimeout(function() {
        window.updateShopCategoryIndicator();
        window.updateCreateViewportHeight();
    }, 350);
})();

window.addEventListener('resize', function() {
    window.updateShopCategoryIndicator();
});
function bannerHex(value, fallback) {
    const cleaned = String(value || '').trim().replace(/^#/, '');
    if (/^[0-9a-fA-F]{6}$/.test(cleaned)) return '#' + cleaned.toUpperCase();
    return fallback;
}

window.bannerFilterCSS = function(effect) {
    if (effect === 'blur') return { filter: 'blur(8px)', transform: 'scale(1.05)', animation: '' };
    if (effect === 'darken') return { filter: 'brightness(0.65)', transform: '', animation: '' };
    if (effect === 'zoom') return { filter: '', transform: '', animation: 'prfbnr-zoom 18s cubic-bezier(0.25, 1, 0.5, 1) infinite alternate' };
    if (effect === 'drift') return { filter: '', transform: '', animation: 'prfbnr-drift 24s linear infinite alternate' };
    return { filter: '', transform: '', animation: '' };
};

window.bannerFxLayer = function() {
    return document.getElementById('settingsBannerFx') || document.getElementById('settingsBannerPreview');
};

window.applyBannerEffect = function() {
    const layer = window.bannerFxLayer();
    const effectInput = document.getElementById('bannerEffectInput');
    if (!layer || !effectInput) return;
    const fx = window.bannerFilterCSS(effectInput.value);
    layer.style.filter = fx.filter;
    layer.style.transform = fx.transform;
    layer.style.animation = fx.animation;
    document.querySelectorAll('[data-bnreff]').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.bnreff === effectInput.value);
    });
};

window.headerDirty = false;
window.headerSaved = null;
window.headerLeaveProc = null;

window.markHeaderDirty = function() {
    window.headerDirty = true;
};

window.selectBannerType = function(value) {
    const typeInput = document.getElementById('bannerTypeInput');
    if (!typeInput) return;
    typeInput.value = String(value || 'color');
    window.markHeaderDirty();
    typeInput.dispatchEvent(new Event('change', { bubbles: true }));
};

window.selectBannerEffect = function(value) {
    const effectInput = document.getElementById('bannerEffectInput');
    if (!effectInput) return;
    effectInput.value = String(value || 'none');
    window.markHeaderDirty();
    window.applyBannerEffect();
};

window.setBannerAngle = function(deg) {
    const angleEl = document.getElementById('bannerAngle');
    if (!angleEl) return;
    let angle = parseInt(deg, 10);
    if (isNaN(angle)) angle = 135;
    angle = Math.min(360, Math.max(0, angle));
    angleEl.value = angle;
    window.markHeaderDirty();
    window.updateBannerPreview();
};

window.syncBannerAnglePresets = function() {
    const angleEl = document.getElementById('bannerAngle');
    const current = angleEl ? String(parseInt(angleEl.value, 10)) : '';
    document.querySelectorAll('[data-bnrangle]').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.bnrangle === current);
    });
};

window.bannerPickerSlot = null;

window.syncBannerStops = function() {
    const a = bannerHex(document.getElementById('bannerColorA')?.value, '#7423CB');
    const b = bannerHex(document.getElementById('bannerColorB')?.value, '#912ADF');
    const typeInput = document.getElementById('bannerTypeInput');
    const isGradient = !!typeInput && typeInput.value === 'gradient';
    const bar = document.getElementById('bannerGradientBar');
    if (bar) bar.style.background = isGradient ? `linear-gradient(90deg, ${a}, ${b})` : a;
    const swA = document.getElementById('bannerSwatchA');
    if (swA) swA.style.background = a;
    const swB = document.getElementById('bannerSwatchB');
    if (swB) swB.style.background = b;
    const stopB = document.getElementById('bannerStopB');
    if (stopB) stopB.style.display = isGradient ? '' : 'none';
};

window.toggleBannerPicker = function(slot) {
    if (slot === 'B') {
        const typeInput = document.getElementById('bannerTypeInput');
        if (!typeInput || typeInput.value !== 'gradient') return;
    }
    const pop = document.getElementById('bannerPickerPop' + slot);
    if (!pop) return;
    const other = document.getElementById(slot === 'A' ? 'bannerPickerPopB' : 'bannerPickerPopA');
    if (other) other.classList.remove('open');
    if (pop.classList.contains('open')) {
        pop.classList.remove('open');
        window.bannerPickerSlot = null;
    } else {
        pop.classList.add('open');
        window.bannerPickerSlot = slot;
    }
    const open = pop.classList.contains('open');
    document.querySelectorAll('.bnrstop').forEach(el => {
        el.classList.toggle('active', open && el.id === 'bannerStop' + window.bannerPickerSlot);
    });
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.bannerDrafts = { color: null, gradient: null, image: null };
window.bannerPrevType = null;
window.bannerImageObjectURL = null;
window.bannerPickerA = null;
window.bannerPickerB = null;

window.bannerColorState = function() {
    const a = document.getElementById('bannerColorA');
    const b = document.getElementById('bannerColorB');
    const angle = document.getElementById('bannerAngle');
    return { a: a ? a.value : '', b: b ? b.value : '', angle: angle ? angle.value : '' };
};

window.bannerSnapshot = function(type) {
    if (type === 'image') {
        const file = document.getElementById('bannerImageFile')?.files?.[0];
        const nameEl = document.getElementById('bannerImageName');
        const name = file ? file.name : (nameEl ? nameEl.textContent : '');
        window.bannerDrafts.image = name ? { name: name } : null;
    } else if (type === 'gradient') {
        const s = window.bannerColorState();
        window.bannerDrafts.gradient = { a: s.a, b: s.b, angle: s.angle };
    } else if (type === 'color') {
        window.bannerDrafts.color = { a: window.bannerColorState().a };
    }
};

window.bannerSyncPickers = function(a, b) {
    if (window.bannerPickerA && a) window.bannerPickerA.setColor(bannerHex(a, '#7423CB'));
    if (window.bannerPickerB && b) window.bannerPickerB.setColor(bannerHex(b, '#912ADF'));
};

window.bannerRestore = function(type) {
    const draft = window.bannerDrafts[type];
    if (!draft) return;
    if (type === 'image') {
        const file = document.getElementById('bannerImageFile')?.files?.[0];
        const nameEl = document.getElementById('bannerImageName');
        const msgEl = document.getElementById('bannerImageMsg');
        if (file) {
            if (nameEl) nameEl.textContent = file.name;
            if (msgEl) msgEl.textContent = 'Choose a different file';
        } else if (draft.name) {
            if (nameEl) nameEl.textContent = draft.name;
            if (msgEl) msgEl.textContent = 'Choose a different file';
        }
    } else {
        const a = document.getElementById('bannerColorA');
        const b = document.getElementById('bannerColorB');
        const angle = document.getElementById('bannerAngle');
        if (a && draft.a) a.value = draft.a;
        if (type === 'gradient') {
            if (b && draft.b) b.value = draft.b;
            if (angle && draft.angle !== undefined) angle.value = draft.angle;
        }
        window.bannerSyncPickers(a ? a.value : '', b ? b.value : '');
    }
};

window.bannerCacheImageURL = function() {
    const file = document.getElementById('bannerImageFile')?.files?.[0];
    if (!file) return null;
    if (!window.bannerImageObjectURL) window.bannerImageObjectURL = URL.createObjectURL(file);
    return window.bannerImageObjectURL;
};

window.bannerRevokeImageURL = function() {
    if (window.bannerImageObjectURL) {
        URL.revokeObjectURL(window.bannerImageObjectURL);
        window.bannerImageObjectURL = null;
    }
};

window.updateBannerPreview = function() {
    const preview = document.getElementById('settingsBannerPreview');
    const typeInput = document.getElementById('bannerTypeInput');
    const layer = window.bannerFxLayer();
    if (!preview || !typeInput || !layer) return;
    const type = typeInput.value;
    if (type === 'gradient') {
        const a = bannerHex(document.getElementById('bannerColorA')?.value, '#7423CB');
        const b = bannerHex(document.getElementById('bannerColorB')?.value, '#912ADF');
        let angle = parseInt(document.getElementById('bannerAngle')?.value, 10);
        if (isNaN(angle)) angle = 135;
        angle = Math.min(360, Math.max(0, angle));
        layer.style.background = `linear-gradient(${angle}deg, ${a}, ${b})`;
    } else if (type === 'image') {
        const url = window.bannerCacheImageURL();
        if (url) {
            layer.style.background = `url('${url}') center/cover no-repeat`;
        }
    } else {
        const a = bannerHex(document.getElementById('bannerColorA')?.value, '#7423CB');
        layer.style.background = a;
    }
    window.syncBannerAnglePresets();
    window.syncBannerStops();
    window.applyBannerEffect();
};

window.updateBannerTypeRows = function() {
    const typeInput = document.getElementById('bannerTypeInput');
    if (!typeInput) return;
    const nextType = typeInput.value;
    if (window.bannerPrevType && window.bannerPrevType !== nextType) {
        window.bannerSnapshot(window.bannerPrevType);
    }
    const isImage = nextType === 'image';
    const isGradient = nextType === 'gradient';
    const rowA = document.getElementById('bannerColorARow');
    const rowB = document.getElementById('bannerColorBRow');
    const rowAngle = document.getElementById('bannerAngleRow');
    const rowImage = document.getElementById('bannerImageRow');
    if (rowA) rowA.style.display = isImage ? 'none' : '';
    if (rowB) rowB.style.display = isGradient ? '' : 'none';
    if (rowAngle) rowAngle.style.display = isGradient ? '' : 'none';
    if (rowImage) rowImage.style.display = isImage ? 'flex' : 'none';
    ['bannerColorA', 'bannerColorB', 'bannerAngle'].forEach(id => {
        const el = document.getElementById(id);
        if (!el) return;
        if (id === 'bannerColorA') {
            el.disabled = isImage;
        } else {
            el.disabled = !isGradient;
        }
    });
    if (window.bannerPrevType && window.bannerPrevType !== nextType) {
        window.bannerRestore(nextType);
    }
    window.bannerPrevType = nextType;
    document.querySelectorAll('[data-bnrtype]').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.bnrtype === nextType);
    });
    const colorALabel = document.getElementById('bannerColorALabel');
    if (colorALabel) colorALabel.textContent = isGradient ? 'Gradient' : 'Color';
    ['bannerPickerPopA', 'bannerPickerPopB'].forEach(id => {
        const pop = document.getElementById(id);
        if (pop) pop.classList.remove('open');
    });
    window.bannerPickerSlot = null;
    document.querySelectorAll('.bnrstop').forEach(el => el.classList.remove('active'));
    window.updateBannerPreview();
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.setBannerFormMsg = function(message, isError) {
    const msg = document.getElementById('bannerFormMsg');
    if (!msg) return;
    msg.classList.remove('visible', 'frminsuc', 'frminerr');
    if (!message) {
        msg.textContent = '';
        return;
    }
    msg.classList.add(isError ? 'frminerr' : 'frminsuc');
    msg.textContent = message;
    requestAnimationFrame(() => msg.classList.add('visible'));
    setTimeout(() => msg.classList.remove('visible'), 3000);
};

window.refreshBannerPreviewAfterSubmit = function(data) {
    const preview = document.getElementById('settingsBannerPreview');
    const typeInput = document.getElementById('bannerTypeInput');
    if (preview && data) {
        if (typeInput && typeInput.value === 'image' && data.banner_id) {
            window.bannerRevokeImageURL();
            const layer = window.bannerFxLayer();
            if (layer) layer.style.background = `url('/static/uploads/banners/${data.banner_id}.png') center/cover no-repeat`;
            window.applyBannerEffect();
        } else {
            window.updateBannerPreview();
        }
    }
    const fileInput = document.getElementById('bannerImageFile');
    if (fileInput) fileInput.value = '';
    const fileName = document.getElementById('bannerImageName');
    if (fileName) fileName.textContent = '';
    const fileMsg = document.getElementById('bannerImageMsg');
    if (fileMsg) fileMsg.textContent = 'Choose a PNG or JPEG';
    window.bannerSnapshot(typeInput ? typeInput.value : 'color');
    const pendingNote = document.getElementById('bannerPendingNote');
    if (data && data.banner_status === 'pending') {
        if (!pendingNote && preview) {
            const note = document.createElement('div');
            note.className = 'feedemt';
            note.id = 'bannerPendingNote';
            note.innerHTML = '<i class="fa-solid fa-hourglass-half"></i> Awaiting admin approval.';
            const wrap = preview.closest('.bnrprevwrap');
            if (wrap && wrap.parentNode) wrap.after(note);
            else preview.after(note);
        }
    } else if (pendingNote) {
        pendingNote.remove();
    }
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
    const form = document.getElementById('bannerForm');
    if (form && !document.getElementById('bannerRemoveBtn')) {
        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'primary hpydng hpyinl hpysm';
        removeBtn.id = 'bannerRemoveBtn';
        removeBtn.innerHTML = '<span>Remove Banner</span>';
        removeBtn.addEventListener('click', window.removeProfileBanner);
        const actions = document.getElementById('bannerActions');
        if (actions) actions.appendChild(removeBtn);
        else form.appendChild(removeBtn);
    }
};

window.removeProfileBanner = function() {
    fetch('/settings/banner/remove', {
        method: 'POST',
        headers: { 'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest' }
    })
    .then(res => res.json())
    .then(data => {
        if (data.error) {
            window.setBannerFormMsg(data.error, true);
            return;
        }
        const layer = window.bannerFxLayer();
        if (layer) {
            layer.style.background = '#D4D4D4';
            layer.style.filter = '';
            layer.style.transform = '';
            layer.style.animation = '';
        }
        const note = document.getElementById('bannerPendingNote');
        if (note) note.remove();
        const removeBtn = document.getElementById('bannerRemoveBtn');
        if (removeBtn) removeBtn.remove();
        window.bannerRevokeImageURL();
        window.bannerDrafts = { color: null, gradient: null, image: null };
        const fileInput = document.getElementById('bannerImageFile');
        if (fileInput) fileInput.value = '';
        const fileName = document.getElementById('bannerImageName');
        if (fileName) fileName.textContent = '';
        const fileMsg = document.getElementById('bannerImageMsg');
        if (fileMsg) fileMsg.textContent = 'Choose a PNG or JPEG';
        window.captureHeaderSaved();
        window.setBannerFormMsg(data.success || 'Profile banner removed', false);
        if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
    })
    .catch(() => {
        window.setBannerFormMsg('An unexpected error occurred. Please try again.', true);
    });
};

window.postBannerForm = function() {
    const form = document.getElementById('bannerForm');
    if (!form) return Promise.resolve(null);
    return fetch(form.action, {
        method: 'POST',
        body: new FormData(form),
        headers: {
            'Accept': 'application/json',
            'X-Requested-With': 'XMLHttpRequest'
        }
    }).then(res => res.json());
};

window.postBannerEffect = function() {
    const effectInput = document.getElementById('bannerEffectInput');
    const body = new URLSearchParams();
    body.set('effect', effectInput ? effectInput.value : 'none');
    return fetch('/settings/banner/effect', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json',
            'X-Requested-With': 'XMLHttpRequest'
        },
        body: body.toString()
    }).then(res => res.json());
};

window.postBannerTextBg = function() {
    const box = document.getElementById('bannerTextBg');
    const body = new URLSearchParams();
    body.set('enabled', box && box.checked ? '1' : '0');
    body.set('color', bannerHex(document.getElementById('bannerTextBgColor')?.value, '#F0F0F0'));
    body.set('border', bannerHex(document.getElementById('bannerTextBgBorder')?.value, '#A9A9A9'));
    body.set('text', bannerHex(document.getElementById('bannerTextBgText')?.value, '#1A202C'));
    body.set('opacity', String(window.bannerTextBgOpacity()));
    return fetch('/settings/banner/textbg', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Accept': 'application/json',
            'X-Requested-With': 'XMLHttpRequest'
        },
        body: body.toString()
    }).then(res => res.json());
};

window.updateHeaderFailed = function(btn, message, after) {
    if (btn) btn.disabled = false;
    window.setBannerFormMsg(message, true);
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
    if (typeof after === 'function') after(false);
};

window.updateHeaderRest = function(btn, after) {
    const done = function(ok) {
        if (typeof after === 'function') after(ok);
    };
    window.postBannerEffect()
    .then(data => {
        if (data && data.error) {
            window.updateHeaderFailed(btn, data.error, after);
            return;
        }
        window.postBannerTextBg()
        .then(textData => {
            if (btn) btn.disabled = false;
            if (textData && textData.error) {
                window.setBannerFormMsg(textData.error, true);
                if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
                done(false);
                return;
            }
            window.captureHeaderSaved();
            window.setBannerFormMsg('Header updated', false);
            if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
            done(true);
        })
        .catch(() => {
            window.updateHeaderFailed(btn, 'An unexpected error occurred. Please try again.', after);
        });
    })
    .catch(() => {
        window.updateHeaderFailed(btn, 'An unexpected error occurred. Please try again.', after);
    });
};

window.updateHeader = function(event, after) {
    if (event) event.preventDefault();
    const btn = document.getElementById('bannerUpdateHeaderBtn');
    if (btn) btn.disabled = true;
    window.setBannerFormMsg('', false);
    const typeInput = document.getElementById('bannerTypeInput');
    const fileInput = document.getElementById('bannerImageFile');
    const hasFile = !!(fileInput && fileInput.files && fileInput.files.length > 0);
    if (typeInput && typeInput.value === 'image' && !hasFile) {
        window.updateHeaderRest(btn, after);
        return false;
    }
    window.postBannerForm()
    .then(data => {
        if (!data) {
            window.updateHeaderRest(btn, after);
            return;
        }
        if (data.error) {
            window.updateHeaderFailed(btn, data.error, after);
            return;
        }
        window.refreshBannerPreviewAfterSubmit(data);
        window.updateHeaderRest(btn, after);
    })
    .catch(() => {
        window.updateHeaderFailed(btn, 'An unexpected error occurred. Please try again.', after);
    });
    return false;
};

window.bannerTextBgOpacity = function() {
    const el = document.getElementById('bannerTextBgOpacity');
    let opacity = parseInt(el ? el.value : '', 10);
    if (isNaN(opacity)) opacity = 78;
    opacity = Math.min(100, Math.max(0, opacity));
    if (el) el.value = opacity;
    window.syncBannerTextBgOpacityVal();
    return opacity;
};

window.syncBannerTextBgOpacityVal = function() {
    const el = document.getElementById('bannerTextBgOpacity');
    const val = document.getElementById('bannerTextBgOpacityVal');
    let p = parseInt(el ? el.value : '', 10);
    if (isNaN(p)) p = 78;
    p = Math.min(100, Math.max(0, p));
    if (val) val.textContent = p + '%';
    if (el) el.style.background = `linear-gradient(90deg, #7423CB ${p}%, #D4D4D4 ${p}%)`;
};

window.updateBannerTextBgOpts = function() {
    const box = document.getElementById('bannerTextBg');
    const opts = document.getElementById('bannerTextBgOpts');
    if (opts) opts.style.display = box && box.checked ? '' : 'none';
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.bannerHexToRgb = function(hex) {
    const m = /^([0-9a-fA-F]{6})$/.exec(String(hex || '').trim().replace(/^#/, ''));
    if (!m) return null;
    return {
        r: parseInt(m[1].slice(0, 2), 16),
        g: parseInt(m[1].slice(2, 4), 16),
        b: parseInt(m[1].slice(4, 6), 16)
    };
};

window.syncBannerTextBgPreview = function() {
    const box = document.getElementById('bannerTextBgPreview');
    if (!box) return;
    const rgb = window.bannerHexToRgb(document.getElementById('bannerTextBgColor')?.value) || { r: 240, g: 240, b: 240 };
    const border = bannerHex(document.getElementById('bannerTextBgBorder')?.value, '#A9A9A9');
    const text = bannerHex(document.getElementById('bannerTextBgText')?.value, '#1A202C');
    let opacity = parseInt(document.getElementById('bannerTextBgOpacity')?.value, 10);
    if (isNaN(opacity)) opacity = 78;
    opacity = Math.min(100, Math.max(0, opacity));
    box.style.background = `rgba(${rgb.r},${rgb.g},${rgb.b},${(opacity / 100).toFixed(2)})`;
    box.style.borderColor = border;
    const sample = box.querySelector('span');
    if (sample) sample.style.color = text;
};

window.setBannerTextBgOpacity = function(value) {
    const el = document.getElementById('bannerTextBgOpacity');
    if (!el) return;
    let opacity = parseInt(value, 10);
    if (isNaN(opacity)) opacity = 78;
    el.value = Math.min(100, Math.max(0, opacity));
    window.syncBannerTextBgOpacityVal();
    window.syncBannerTextBgOpacityPresets();
    window.syncBannerTextBgPreview();
};

window.syncBannerTextBgOpacityPresets = function() {
    const el = document.getElementById('bannerTextBgOpacity');
    const current = el ? String(parseInt(el.value, 10)) : '';
    document.querySelectorAll('[data-bnrop]').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.bnrop === current);
    });
};

window.bannerTextBgPickers = {};

window.bannerTextBgPopId = function(slot) {
    if (slot === 'border') return 'bannerTextBgBorderPickerPop';
    if (slot === 'text') return 'bannerTextBgTextPickerPop';
    return 'bannerTextBgPickerPop';
};

window.toggleBannerTextBgPicker = function(slot) {
    const pop = document.getElementById(window.bannerTextBgPopId(slot));
    if (!pop) return;
    ['bg', 'border', 'text'].forEach(other => {
        if (other === slot) return;
        const otherPop = document.getElementById(window.bannerTextBgPopId(other));
        if (otherPop) otherPop.classList.remove('open');
    });
    pop.classList.toggle('open');
    window.syncBannerTextBgStopActives();
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.syncBannerTextBgStopActives = function() {
    [
        { stop: 'bannerTextBgStopBg', pop: 'bannerTextBgPickerPop' },
        { stop: 'bannerTextBgStopBorder', pop: 'bannerTextBgBorderPickerPop' },
        { stop: 'bannerTextBgStopText', pop: 'bannerTextBgTextPickerPop' }
    ].forEach(def => {
        const stop = document.getElementById(def.stop);
        const pop = document.getElementById(def.pop);
        if (stop && pop) stop.classList.toggle('active', pop.classList.contains('open'));
    });
};

window.initBannerTextBgPickers = function() {
    if (typeof window.GlobalColorPicker !== 'function') return;
    [
        { key: 'bg', box: 'bannerTextBgPicker', input: 'bannerTextBgColor', swatch: 'bannerTextBgSwatch', fallback: '#F0F0F0' },
        { key: 'border', box: 'bannerTextBgBorderPicker', input: 'bannerTextBgBorder', swatch: 'bannerTextBgBorderSwatch', fallback: '#A9A9A9' },
        { key: 'text', box: 'bannerTextBgTextPicker', input: 'bannerTextBgText', swatch: 'bannerTextBgTextSwatch', fallback: '#1A202C' }
    ].forEach(def => {
        const box = document.getElementById(def.box);
        const input = document.getElementById(def.input);
        const swatch = document.getElementById(def.swatch);
        if (!box || !input || box.dataset.pickerInit) return;
        box.dataset.pickerInit = '1';
        const picker = new window.GlobalColorPicker(box, {
            width: 170,
            initialColor: bannerHex(input.value, def.fallback),
            onChange: function(hex) {
                input.value = String(hex).toUpperCase();
                if (swatch) swatch.style.background = bannerHex(input.value, def.fallback);
                window.markHeaderDirty();
                window.syncBannerTextBgPreview();
            }
        });
        window.bannerTextBgPickers[def.key] = picker;
    });
};

window.closeBannerPickers = function() {
    const pane = document.getElementById('bannerPaneBanner');
    if (!pane) return;
    pane.querySelectorAll('.bnrpop.open').forEach(p => p.classList.remove('open'));
    pane.querySelectorAll('.bnrstop.active').forEach(s => s.classList.remove('active'));
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.captureHeaderSaved = function() {
    const layer = window.bannerFxLayer();
    const box = document.getElementById('bannerTextBg');
    window.headerSaved = {
        type: document.getElementById('bannerTypeInput')?.value || 'color',
        colorA: document.getElementById('bannerColorA')?.value || '',
        colorB: document.getElementById('bannerColorB')?.value || '',
        angle: document.getElementById('bannerAngle')?.value || '',
        effect: document.getElementById('bannerEffectInput')?.value || 'none',
        textBg: !!(box && box.checked),
        textBgColor: document.getElementById('bannerTextBgColor')?.value || '',
        textBgBorder: document.getElementById('bannerTextBgBorder')?.value || '',
        textBgText: document.getElementById('bannerTextBgText')?.value || '',
        textBgOpacity: document.getElementById('bannerTextBgOpacity')?.value || '',
        layerBg: layer ? layer.style.background : ''
    };
    window.headerDirty = false;
};

window.revertHeaderChanges = function() {
    const s = window.headerSaved;
    if (!s) return;
    window.bannerDrafts = { color: null, gradient: null, image: null };
    window.bannerPrevType = null;
    const setVal = function(id, value) {
        const el = document.getElementById(id);
        if (el) el.value = value;
    };
    setVal('bannerTypeInput', s.type);
    setVal('bannerColorA', s.colorA);
    setVal('bannerColorB', s.colorB);
    setVal('bannerAngle', s.angle);
    setVal('bannerEffectInput', s.effect);
    const box = document.getElementById('bannerTextBg');
    if (box) box.checked = s.textBg;
    setVal('bannerTextBgColor', s.textBgColor);
    setVal('bannerTextBgBorder', s.textBgBorder);
    setVal('bannerTextBgText', s.textBgText);
    setVal('bannerTextBgOpacity', s.textBgOpacity);
    window.bannerRevokeImageURL();
    const fileInput = document.getElementById('bannerImageFile');
    if (fileInput) fileInput.value = '';
    const fileName = document.getElementById('bannerImageName');
    if (fileName) fileName.textContent = '';
    const fileMsg = document.getElementById('bannerImageMsg');
    if (fileMsg) fileMsg.textContent = 'Choose a PNG or JPEG';
    window.bannerSyncPickers(s.colorA, s.colorB);
    const stored = window.bannerTextBgPickers || {};
    if (stored.bg) stored.bg.setColor(bannerHex(s.textBgColor, '#F0F0F0'));
    if (stored.border) stored.border.setColor(bannerHex(s.textBgBorder, '#A9A9A9'));
    if (stored.text) stored.text.setColor(bannerHex(s.textBgText, '#1A202C'));
    window.closeBannerPickers();
    window.updateBannerTypeRows();
    window.applyBannerEffect();
    const layer = window.bannerFxLayer();
    if (layer) layer.style.background = s.layerBg;
    window.updateBannerTextBgOpts();
    window.syncBannerTextBgOpacityVal();
    window.syncBannerTextBgOpacityPresets();
    window.syncBannerTextBgPreview();
    window.headerDirty = false;
};

window.confirmLeaveHeader = function(proceed) {
    window.headerLeaveProc = (typeof proceed === 'function') ? proceed : null;
    if (typeof window.showModal !== 'function') {
        const proc = window.headerLeaveProc;
        window.headerLeaveProc = null;
        if (proc) proc();
        return;
    }
    window.showModal({
        title: 'Wait there!',
        bodyStyle: 'gap: 16px;',
        onClose: function() { window.headerLeaveProc = null; },
        body: `
            <p style="font-family: 'Ubuntu', sans-serif; font-size: 14px; color: #3D3D3D; line-height: 1.5; margin: 0;">
                You have unsaved changes. Would you like to proceed and lose changes, or save them?
            </p>
            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px;">
                <button type="button" class="primary hpyerr hpyinl hpysm" onclick="window.discardHeaderAndLeave()">
                    <span>Don't save</span>
                </button>
                <button type="button" class="primary hpyprim hpyinl hpysm" onclick="window.saveHeaderAndLeave()">
                    <span>Save</span>
                </button>
            </div>
        `
    });
};

window.saveHeaderAndLeave = function() {
    const proc = window.headerLeaveProc;
    window.updateHeader(null, function(ok) {
        window.closeModal();
        if (ok && proc) proc();
    });
};

window.discardHeaderAndLeave = function() {
    const proc = window.headerLeaveProc;
    window.headerLeaveProc = null;
    window.revertHeaderChanges();
    window.closeModal();
    if (proc) proc();
};

window.wrapSettingsTabSwitch = function() {
    if (window.__bannerSettingsTabWrapped) return;
    if (typeof window.switchSettingsTab !== 'function') return;
    window.__bannerSettingsTabWrapped = true;
    const orig = window.switchSettingsTab;
    window.switchSettingsTab = function(tabName, event) {
        const profileActive = document.getElementById('tab-profile')?.classList.contains('active');
        const clicked = event ? event.currentTarget : null;
        if (profileActive && window.headerDirty && clicked && !clicked.classList.contains('active')) {
            if (event && event.preventDefault) event.preventDefault();
            window.confirmLeaveHeader(function() {
                orig(tabName, { currentTarget: clicked, preventDefault: function() {} });
            });
            return;
        }
        orig(tabName, event);
    };
};

window.switchBannerTab = function(tab, event) {
    if (event) event.preventDefault();
    const card = document.getElementById('bannerTabInd');
    if (!card) return;
    const scope = card.closest('.setcrd') || document;
    const tabs = Array.from(scope.querySelectorAll('.prftabbtn'));
    const clicked = event ? event.currentTarget : null;
    if (!clicked || clicked.classList.contains('active')) return;
    const banner = document.getElementById('bannerPaneBanner');
    const leavingBanner = !!banner && banner.style.display !== 'none' && tab !== 'banner';
    if (leavingBanner && window.headerDirty) {
        window.confirmLeaveHeader(function() { window.doSwitchBannerTab(tab, clicked); });
        return;
    }
    window.doSwitchBannerTab(tab, clicked);
};

window.doSwitchBannerTab = function(tab, clicked) {
    const card = document.getElementById('bannerTabInd');
    const scope = card ? (card.closest('.setcrd') || document) : document;
    const tabs = Array.from(scope.querySelectorAll('.prftabbtn'));
    let target = (clicked && clicked.isConnected) ? clicked : null;
    if (!target) {
        target = tabs.find(t => (t.getAttribute('onclick') || '').includes(`'${tab}'`)) || null;
    }
    tabs.forEach(t => t.classList.remove('active'));
    if (target) target.classList.add('active');
    const ind = document.getElementById('bannerTabInd');
    if (ind && target) {
        ind.style.left = target.offsetLeft + 'px';
        ind.style.width = target.offsetWidth + 'px';
        ind.style.height = target.offsetHeight + 'px';
        ind.style.top = target.offsetTop + 'px';
    }
    const profile = document.getElementById('bannerPaneProfile');
    const banner = document.getElementById('bannerPaneBanner');
    if (profile) profile.style.display = tab === 'profile' ? '' : 'none';
    if (banner) banner.style.display = tab === 'banner' ? '' : 'none';
    if (tab === 'banner') {
        window.syncBannerTextBgOpacityVal();
        window.syncBannerTextBgOpacityPresets();
        window.syncBannerTextBgPreview();
        window.updateBannerPreview();
    }
    if (typeof window.updateViewportHeight === 'function') window.updateViewportHeight();
};

window.updateBannerTabIndicator = function() {
    const ind = document.getElementById('bannerTabInd');
    if (!ind) return;
    const scope = ind.closest('.setcrd') || document;
    const active = scope.querySelector('.prftabbtn.active');
    if (active) {
        ind.style.left = active.offsetLeft + 'px';
        ind.style.width = active.offsetWidth + 'px';
        ind.style.height = active.offsetHeight + 'px';
        ind.style.top = active.offsetTop + 'px';
    }
};



window.initBannerColorPickers = function() {
    if (typeof window.GlobalColorPicker !== 'function') return;
    const defs = [
        { box: 'bannerColorAPicker', input: 'bannerColorA', fallback: '#7423CB', slot: 'A' },
        { box: 'bannerColorBPicker', input: 'bannerColorB', fallback: '#912ADF', slot: 'B' }
    ];
    defs.forEach(def => {
        const box = document.getElementById(def.box);
        const input = document.getElementById(def.input);
        if (!box || !input || box.dataset.pickerInit) return;
        box.dataset.pickerInit = '1';
        const picker = new window.GlobalColorPicker(box, {
            width: 170,
            initialColor: bannerHex(input.value, def.fallback),
            onChange: function(hex) {
                input.value = String(hex).toUpperCase();
                window.markHeaderDirty();
                window.updateBannerPreview();
            }
        });
        if (def.slot === 'A') window.bannerPickerA = picker;
        else window.bannerPickerB = picker;
    });
};

window.initProfileBanner = function() {
    const typeInput = document.getElementById('bannerTypeInput');
    if (!typeInput || typeInput.dataset.bannerBound) return;
    typeInput.dataset.bannerBound = '1';
    typeInput.addEventListener('change', window.updateBannerTypeRows);
    const textBgBox = document.getElementById('bannerTextBg');
    if (textBgBox) textBgBox.addEventListener('change', () => {
        window.markHeaderDirty();
        window.updateBannerTextBgOpts();
    });
    const textBgOpacity = document.getElementById('bannerTextBgOpacity');
    if (textBgOpacity) textBgOpacity.addEventListener('input', () => {
        window.markHeaderDirty();
        window.syncBannerTextBgOpacityVal();
        window.syncBannerTextBgOpacityPresets();
        window.syncBannerTextBgPreview();
    });
    window.initBannerColorPickers();
    window.initBannerTextBgPickers();
    window.updateBannerTextBgOpts();
    window.syncBannerTextBgOpacityVal();
    window.syncBannerTextBgOpacityPresets();
    window.syncBannerTextBgPreview();
    const angleEl = document.getElementById('bannerAngle');
    if (angleEl) angleEl.addEventListener('input', () => {
        window.markHeaderDirty();
        window.updateBannerPreview();
    });
    const fileInput = document.getElementById('bannerImageFile');
    if (fileInput) {
        fileInput.addEventListener('change', () => {
            window.bannerRevokeImageURL();
            const file = fileInput.files?.[0];
            const nameEl = document.getElementById('bannerImageName');
            const msgEl = document.getElementById('bannerImageMsg');
            if (nameEl) nameEl.textContent = file ? file.name : '';
            if (msgEl) msgEl.textContent = file ? 'Choose a different file' : 'Choose a PNG or JPEG';
            window.markHeaderDirty();
            window.updateBannerPreview();
        });
    }
    const form = document.getElementById('bannerForm');
    if (form) form.addEventListener('submit', window.updateHeader);
    window.updateBannerTypeRows();
    window.updateBannerTabIndicator();
    window.captureHeaderSaved();
    window.wrapSettingsTabSwitch();
};

if (!window.__bannerResizeBound) {
    window.__bannerResizeBound = true;
    window.addEventListener('resize', window.updateBannerTabIndicator);
}

if (!window.__bannerOutsideBound) {
    window.__bannerOutsideBound = true;
    document.addEventListener('click', function(event) {
        if (!event.target || !event.target.closest) return;
        if (event.target.closest('.bnrpop') || event.target.closest('.bnrstop')) return;
        window.closeBannerPickers();
    });
}

if (!window.__bannerLeaveBound) {
    window.__bannerLeaveBound = true;
    document.addEventListener('htmx:confirm', function(event) {
        if (!window.headerDirty) return;
        if (!document.getElementById('bannerPaneBanner')) return;
        if (event && event.preventDefault) event.preventDefault();
        const detail = event ? event.detail : null;
        window.confirmLeaveHeader(function() {
            if (detail && typeof detail.issueRequest === 'function') detail.issueRequest();
        });
    });
    window.addEventListener('beforeunload', function(event) {
        if (!window.headerDirty) return;
        if (!document.getElementById('bannerPaneBanner')) return;
        event.preventDefault();
        event.returnValue = '';
    });
}

const initAssetsPage = () => {
    document.querySelectorAll('#assetGrid img.avtldimg').forEach(img => {
        if (img.complete && img.naturalWidth > 0) {
            img.classList.add('ld');
        } else {
            img.onload = () => img.classList.add('ld');
        }
    });

    const searchInput = document.getElementById('assetSearch');
    if (searchInput) {
        searchInput.oninput = function() {
            const q = this.value.trim().toLowerCase();
            document.querySelectorAll('#assetGrid .shp-card, #assetGrid .avtitm').forEach(item => {
                const nameEl = item.querySelector('.avtitmnm');
                const match = !q || (nameEl && nameEl.textContent.toLowerCase().indexOf(q) !== -1);
                item.style.display = match ? '' : 'none';
            });
        };
    }
};

function assetEsc(s) {
    if (window.escapeHTML) return window.escapeHTML(String(s == null ? '' : s));
    return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
}

function assetFormatTime(seconds) {
    if (isNaN(seconds) || seconds < 0) return '0:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return mins + ':' + (secs < 10 ? '0' : '') + secs;
}

function getAssetAudio() {
    if (!window.assetAudio) {
        window.assetAudio = new Audio();
        window.assetAudio.volume = 0.5;
        window.assetAudio.preload = 'metadata';
        window.assetAudio.addEventListener('timeupdate', updateAssetPlayerProgress);
        window.assetAudio.addEventListener('loadedmetadata', updateAssetPlayerProgress);
        window.assetAudio.addEventListener('ended', () => updateAssetPlayerPlaying(false));
        window.assetAudio.addEventListener('play', () => {
            updateAssetPlayerPlaying(true);
            if (window.currentAudio && !window.currentAudio.paused) {
                try { window.currentAudio.pause(); } catch (e) {}
            }
        });
        window.assetAudio.addEventListener('pause', () => updateAssetPlayerPlaying(false));
        if (window.currentAudio && !window.currentAudio._assetPauseHook) {
            window.currentAudio._assetPauseHook = true;
            window.currentAudio.addEventListener('play', () => {
                if (window.assetAudio && !window.assetAudio.paused) {
                    try { window.assetAudio.pause(); } catch (e) {}
                }
            });
        }
    }
    return window.assetAudio;
}

function updateAssetPlayerProgress() {
    const audio = window.assetAudio;
    if (!audio) return;
    const cur = audio.currentTime || 0;
    let dur = audio.duration;
    if (isNaN(dur) || dur <= 0) dur = 0;
    const pct = dur > 0 ? (Math.min(100, Math.max(0, (cur / dur) * 100)) + '%') : '0%';
    const fill = document.getElementById('astMusBarFill');
    if (fill) fill.style.setProperty('width', pct, 'important');
    const curEl = document.getElementById('astMusCurTime');
    if (curEl) curEl.textContent = assetFormatTime(cur);
    const durEl = document.getElementById('astMusDurTime');
    if (durEl) durEl.textContent = assetFormatTime(dur);
}

function updateAssetPlayerPlaying(isPlaying) {
    const icon = document.getElementById('astMusPlayIcon');
    if (icon) icon.className = isPlaying ? 'fa-solid fa-pause' : 'fa-solid fa-play';
}

function pauseAssetAudio() {
    if (window.assetAudio && !window.assetAudio.paused) {
        try { window.assetAudio.pause(); } catch (e) {}
    }
}

window.toggleAssetTrack = function() {
    const audio = getAssetAudio();
    if (!audio.src && window.assetPreviewSrc) audio.src = window.assetPreviewSrc;
    if (audio.paused) {
        audio.play().catch(() => updateAssetPlayerPlaying(false));
    } else {
        audio.pause();
    }
};

window.seekAssetTrack = function(event) {
    const audio = window.assetAudio;
    if (!audio || !audio.duration || isNaN(audio.duration)) return;
    const bar = event.currentTarget;
    if (!bar) return;
    const rect = bar.getBoundingClientRect();
    const pct = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
    audio.currentTime = pct * audio.duration;
};

window.toggleMuteAssetTrack = function() {
    const audio = getAssetAudio();
    audio.muted = !audio.muted;
    const icon = document.getElementById('astMusVolIcon');
    if (icon) icon.className = audio.muted ? 'fa-solid fa-volume-xmark' : 'fa-solid fa-volume-high';
};

window.expandAsset = function(card) {
    if (!card) return;
    if (!window.showModal) return;
    const id = card.getAttribute('data-id');
    if (!id) return;
    const type = (card.getAttribute('data-type') || '').toLowerCase();
    const nameEl = card.querySelector('.avtitmnm');
    const metaEl = card.querySelector('.avtitmcr');
    const name = nameEl ? nameEl.textContent.trim() : ('Asset #' + id);
    const meta = metaEl ? metaEl.textContent.trim() : '';
    const safeName = assetEsc(name);
    const safeMeta = assetEsc(meta);

    if (type !== 'sound') pauseAssetAudio();

    if (type === 'sound') {
        const fileUrl = '/api/v1/assets/' + encodeURIComponent(id) + '/file';
        const audio = getAssetAudio();
        if (String(window.assetPreviewId || '') !== String(id)) {
            try { audio.src = fileUrl; } catch (e) {}
        }
        window.assetPreviewSrc = fileUrl;
        window.assetPreviewId = String(id);
        window.showModal({
            title: name,
            subtitle: meta,
            body: '<div class="shp-modal-item">'
                + '<div class="musplayer-container" style="display:flex;">'
                + '<div class="musplayer">'
                + '<div class="musplayer-cvr" style="display:flex;align-items:center;justify-content:center;"><i class="fa-solid fa-music" style="font-size:14px;color:#3D3D3D;opacity:0.7;"></i></div>'
                + '<div class="musplayer-meta"><span id="astMusTitle" class="musplayer-ttl">' + safeName + '</span><span class="musplayer-art">' + safeMeta + '</span></div>'
                + '<button type="button" class="musplayer-plybtn" onclick="window.toggleAssetTrack()" title="Play/Pause"><i id="astMusPlayIcon" class="fa-solid fa-play"></i></button>'
                + '<div class="musplayer-seek"><span id="astMusCurTime" class="musplayer-time">0:00</span>'
                + '<div class="musplayer-barcnt" onclick="window.seekAssetTrack(event)"><div id="astMusBarFill" class="musplayer-barfill"></div></div>'
                + '<span id="astMusDurTime" class="musplayer-time">0:00</span></div>'
                + '<button type="button" class="musplayer-volbtn" onclick="window.toggleMuteAssetTrack()" title="Mute/Unmute"><i id="astMusVolIcon" class="fa-solid fa-volume-high"></i></button>'
                + '</div></div>'
                + '<div class="shp-modal-details"><p class="shp-modal-desc">ID: ' + assetEsc(id) + '</p></div></div>',
            onClose: pauseAssetAudio
        });
        updateAssetPlayerProgress();
        updateAssetPlayerPlaying(!audio.paused);
        if (audio.paused) {
            audio.play().catch(() => updateAssetPlayerPlaying(false));
        }
        return;
    }

    const isMesh = type === 'mesh';
    const imgUrl = isMesh
        ? '/api/v1/assets/' + encodeURIComponent(id) + '/render'
        : '/api/v1/assets/' + encodeURIComponent(id) + '/file';
    window.showModal({
        title: name,
        subtitle: meta,
        body: '<div class="shp-modal-item">'
            + '<div class="shp-modal-imgwrap" style="width:100%;height:auto;max-height:60vh;min-height:240px;aspect-ratio:auto;">'
            + '<img src="' + imgUrl + '" alt="' + safeName + '" style="width:100%;height:100%;max-height:60vh;min-height:240px;object-fit:contain;" onerror="this.src=\'/static/useful/temp/pfp.png\';">'
            + '</div>'
            + '<div class="shp-modal-details"><p class="shp-modal-desc">ID: ' + assetEsc(id) + '</p></div></div>'
    });
};

function stopAssetAudioOnNavigation() {
    pauseAssetAudio();
}

window.openShopItem = function(card) {
    if (!card) return;
    const id = card.getAttribute('data-id');
    if (!id) return;
    window.navigateWithHistory('/shop/item/' + encodeURIComponent(id));
};

if (!window.__assetNavBound) {
    window.__assetNavBound = true;
    document.addEventListener('click', function(event) {
        const card = event.target.closest ? event.target.closest('.shp-card[data-goto="shop-item"]') : null;
        if (card && !event.target.closest('button') && !event.target.closest('a')) {
            window.openShopItem(card);
        }
    });

    document.addEventListener('keydown', function(event) {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        const card = event.target.closest ? event.target.closest('.shp-card[data-goto="shop-item"]') : null;
        if (card) {
            event.preventDefault();
            window.openShopItem(card);
        }
    });
}

window.addEventListener('pagehide', stopAssetAudioOnNavigation);
window.addEventListener('beforeunload', stopAssetAudioOnNavigation);
document.addEventListener('htmx:beforeSwap', stopAssetAudioOnNavigation);

initAssetsPage();
document.addEventListener('htmx:afterSettle', initAssetsPage);

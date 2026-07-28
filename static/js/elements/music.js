let musicNoticeTimer = null;

window.getSpinnerHTML = function() {
    return `
        <div class="musld">
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
        </div>
    `;
};

window.startNoticeTimer = function() {
    const btn = document.getElementById('musicNoticeContinueBtn');
    const btnText = document.getElementById('musicNoticeContinueText');
    if (!btn || !btnText) return;

    if (musicNoticeTimer) {
        clearInterval(musicNoticeTimer);
        musicNoticeTimer = null;
    }

    let timeLeft = 5;
    btn.disabled = true;
    btnText.textContent = `Continue (${timeLeft}s)`;

    musicNoticeTimer = setInterval(() => {
        timeLeft--;
        if (timeLeft > 0) {
            btnText.textContent = `Continue (${timeLeft}s)`;
        } else {
            clearInterval(musicNoticeTimer);
            musicNoticeTimer = null;
            btn.disabled = false;
            btnText.textContent = 'Continue';
        }
    }, 1000);
};

window.openMusicModal = function() {
    if (localStorage.getItem('musicNoticeSeen') === 'true') {
        const searchModal = document.getElementById('musicModal');
        if (searchModal) {
            searchModal.classList.add('active');
            const input = document.getElementById('musSearchInput');
            if (input) input.focus();
        }
        return;
    }

    const noticeModal = document.getElementById('musicNoticeModal');
    if (noticeModal) {
        noticeModal.classList.add('active');
        window.startNoticeTimer();
    }
};

window.closeMusicNoticeModal = function() {
    if (musicNoticeTimer) {
        clearInterval(musicNoticeTimer);
        musicNoticeTimer = null;
    }
    const noticeModal = document.getElementById('musicNoticeModal');
    if (noticeModal) {
        noticeModal.classList.remove('active');
    }
};

window.closeMusicNoticeModalOnOverlay = function(event) {
    if (event.target && event.target.id === 'musicNoticeModal') {
        window.closeMusicNoticeModal();
    }
};

window.proceedToMusicSearch = function() {
    const btn = document.getElementById('musicNoticeContinueBtn');
    if (btn && btn.disabled) return;

    localStorage.setItem('musicNoticeSeen', 'true');
    window.closeMusicNoticeModal();

    const searchModal = document.getElementById('musicModal');
    if (searchModal) {
        searchModal.classList.add('active');
        const input = document.getElementById('musSearchInput');
        if (input) input.focus();
    }
};

window.closeMusicModal = function() {
    const modal = document.getElementById('musicModal');
    if (modal) {
        modal.classList.remove('active');
    }
};

window.closeMusicModalOnOverlay = function(event) {
    if (event.target && event.target.id === 'musicModal') {
        window.closeMusicModal();
    }
};

window.searchMusic = function(event) {
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const input = document.getElementById('musSearchInput');
    const container = document.getElementById('musResults');
    if (!input || !container) return false;

    const query = input.value.trim();
    if (!query) return false;

    container.innerHTML = window.getSpinnerHTML();

    fetch('/api/v1/music/search?q=' + encodeURIComponent(query))
        .then(res => res.json())
        .then(data => {
            if (!data.data || data.data.length === 0) {
                container.innerHTML = '<div class="msempty">No tracks found</div>';
                return;
            }

            let html = '';
            data.data.forEach(track => {
                const cover = (track.album && track.album.cover_small) ? track.album.cover_small : '/static/useful/temp/pfp.png';
                const artist = (track.artist && track.artist.name) ? track.artist.name : 'Unknown Artist';
                const preview = track.preview ? `<audio controls class="musaud" src="${track.preview}"></audio>` : '';

                const safeTitle = window.escapeHTML ? window.escapeHTML(track.title) : track.title;
                const safeArtist = window.escapeHTML ? window.escapeHTML(artist) : artist;

                html += `
                    <div class="musitm">
                        <img src="${cover}" alt="${safeTitle}" class="muscvr">
                        <div class="musinf">
                            <div class="musttl">${safeTitle}</div>
                            <div class="musart">${safeArtist}</div>
                        </div>
                        ${preview}
                    </div>
                `;
            });
            container.innerHTML = html;
        })
        .catch(() => {
            container.innerHTML = '<div class="msempty">Failed to load tracks. Please try again.</div>';
        });

    return false;
};
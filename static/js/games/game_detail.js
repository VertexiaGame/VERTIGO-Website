window.updateGameDetailTabIndicator = function() {
    const indicator = document.querySelector('.gam-det-tabind');
    const activeTab = document.querySelector('.gam-det-tabbtn.active');
    if (indicator && activeTab) {
        indicator.style.left = activeTab.offsetLeft + 'px';
        indicator.style.width = activeTab.offsetWidth + 'px';
        indicator.style.height = activeTab.offsetHeight + 'px';
        indicator.style.top = activeTab.offsetTop + 'px';
    }
};

window.switchGameDetailTab = function(tabName, event) {
    if (event) {
        if (event.currentTarget && event.currentTarget.disabled) return;
        event.preventDefault();
    }
    if (tabName !== 'about' && tabName !== 'servers') return;
    const tabs = Array.from(document.querySelectorAll('.gam-det-tabbtn:not([disabled])'));
    const clickedTab = event ? event.currentTarget : document.querySelector('.gam-det-tabbtn[data-tab="' + tabName + '"]');
    if (clickedTab && clickedTab.classList.contains('active')) return;
    tabs.forEach(tab => tab.classList.remove('active'));
    if (clickedTab) clickedTab.classList.add('active');
    const aboutPane = document.getElementById('gamDetPaneAbout');
    const serversPane = document.getElementById('gamDetPaneServers');
    if (aboutPane) aboutPane.hidden = (tabName !== 'about');
    if (serversPane) serversPane.hidden = (tabName !== 'servers');
    window.updateGameDetailTabIndicator();
    if (tabName === 'servers') {
        window.refreshGameServers(null);
    }
};

window.gameDetailGameId = function() {
    const playBtn = document.getElementById('gamPlayBtn');
    if (playBtn && playBtn.dataset.gameId) return playBtn.dataset.gameId;
    const row = document.getElementById('gamRateRow');
    if (row && row.dataset.gameId) return row.dataset.gameId;
    const m = (window.location.pathname || '').match(/\/play\/(\d+)/);
    return m ? m[1] : '';
};

window.buildClientJoinUri = function(host, port, ukey) {
    return 'vertexia-client://play?ip=' + encodeURIComponent(host || '') + '&port=' + encodeURIComponent(String(port == null ? '' : port)) + '&ukey=' + encodeURIComponent(ukey || '');
};

window.launchClientJoinUri = function(uri) {
    if (!uri) return;
    var frame = document.getElementById('vtxClientLaunchFrame');
    if (!frame) {
        frame = document.createElement('iframe');
        frame.id = 'vtxClientLaunchFrame';
        frame.style.display = 'none';
        frame.setAttribute('aria-hidden', 'true');
        frame.setAttribute('tabindex', '-1');
        document.body.appendChild(frame);
    }
    try {
        frame.src = uri;
    } catch (e) {
        window.location.href = uri;
        return;
    }
    var hideAfterLaunch = function() {
        if (window.hideGlobalLoader) {
            window.hideGlobalLoader();
        } else {
            var bar = document.getElementById('pageProgress');
            if (bar) bar.classList.remove('active');
            var glb = document.getElementById('globalLoader');
            if (glb) {
                glb.classList.remove('visible');
                glb.style.display = 'none';
            }
        }
    };
    hideAfterLaunch();
    setTimeout(hideAfterLaunch, 600);
};

window.joinGame = function(event) {
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const playBtn = document.getElementById('gamPlayBtn');
    if (playBtn && playBtn.dataset.logged !== 'true') {
        window.location.href = '/login?next=' + encodeURIComponent(window.location.pathname);
        return;
    }
    const gameId = window.gameDetailGameId();
    if (!gameId) return;
    if (window.joinGameInFlight) return;
    window.joinGameInFlight = true;
    const playSpan = playBtn ? playBtn.querySelector('span') : null;
    const playOrig = playSpan ? playSpan.innerHTML : '';
    if (playBtn) {
        playBtn.disabled = true;
        playBtn.classList.add('is-launching');
        playBtn.classList.remove('is-launched');
    }
    if (playSpan) playSpan.textContent = 'Launching…';
    const restorePlayBtn = function() {
        window.joinGameInFlight = false;
        if (playBtn) {
            playBtn.disabled = false;
            playBtn.classList.remove('is-launching');
            if (playSpan) playSpan.innerHTML = playOrig;
        }
    };
    const flashLaunched = function() {
        if (!playBtn || !playSpan) return;
        playBtn.classList.remove('is-launching');
        playBtn.classList.add('is-launched');
        playSpan.innerHTML = '<i class="fa-solid fa-check"></i> Launched';
        setTimeout(function() {
            if (!document.getElementById('gamPlayBtn')) return;
            playBtn.classList.remove('is-launched');
            if (playSpan) playSpan.innerHTML = playOrig;
        }, 2200);
    };
    fetch('/api/v1/games/' + encodeURIComponent(gameId) + '/join', {method: 'POST', headers: {'HX-Request': 'true'}})
        .then(function(res) { return res.json().then(function(j) { return {ok: res.ok, status: res.status, body: j}; }); })
        .then(function(out) {
            restorePlayBtn();
            if (!out.ok) {
                if (out.status === 401) {
                    window.location.href = '/login?next=' + encodeURIComponent(window.location.pathname);
                    return;
                }
                const msg = (out.body && out.body.error) || 'Could not join game. Please try again.';
                if (window.showModal) window.showModal({title: 'Could Not Join', body: '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">' + window.escapeHTML(msg) + '</p>'});
                return;
            }
            const d = out.body || {};
            const srv = d.server || {};
            const uri = d.client_uri || window.buildClientJoinUri(srv.host, srv.port, d.ukey);
            window.launchClientJoinUri(uri);
            flashLaunched();
            window.refreshGameServers(null);
        })
        .catch(function() {
            restorePlayBtn();
            if (window.showModal) window.showModal({title: 'Could Not Join', body: '<p style="font-family:Ubuntu,sans-serif;font-size:14px;color:#3D3D3D;margin:0;">Connection lost. Please check your connection and try again.</p>'});
        });
};

window.refreshGameServers = function(event) {
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const list = document.getElementById('gamSrvList');
    if (!list) return;
    const gameId = list.dataset.gameId || window.gameDetailGameId();
    if (!gameId) return;
    fetch('/api/v1/games/' + encodeURIComponent(gameId) + '/servers', {headers: {'HX-Request': 'true'}})
        .then(function(res) { return res.json(); })
        .then(function(data) {
            window.renderGameServers(data);
        })
        .catch(function() {
            list.innerHTML = '<div class="feedemt">Could not load servers.</div>';
        });
};

window.renderGameServerAvatars = function(players) {
    const esc = window.escapeHTML;
    const shown = (players || []).slice(0, 10);
    let html = '';
    shown.forEach(function(p) {
        const name = esc(String((p && p.username) || ''));
        const id = (p && p.id) || 0;
        const src = (p && p.headshot) || ('/api/v1/avatar/headshot/' + encodeURIComponent(String(id)));
        html += '<a href="/user/' + encodeURIComponent(String(id)) + '" hx-get="/user/' + encodeURIComponent(String(id)) + '" hx-target="body" hx-push-url="true" title="' + name + '"><img class="gam-srv-avt" src="' + esc(src) + '" alt="' + name + '" loading="lazy" onerror="this.onerror=null;this.src=\'/static/useful/temp/pfp.png\';"></a>';
    });
    const extra = (players || []).length - shown.length;
    if (extra > 0) {
        html += '<span class="gam-srv-more" title="' + esc(String(extra)) + ' more players">+' + esc(String(extra)) + '</span>';
    }
    return html;
};

window.renderGameServers = function(data) {
    const list = document.getElementById('gamSrvList');
    if (!list) return;
    const servers = (data && data.servers) || [];
    if (servers.length === 0) {
        list.innerHTML = '<div class="feedemt">No active servers right now.</div>';
        return;
    }
    const esc = window.escapeHTML;
    let html = '';
    servers.forEach(function(s) {
        const players = esc(String(s.player_count)) + ' out of ' + esc(String(s.max_players));
        html += '<div class="gam-srv-row">' +
            '<div class="gam-srv-main">' +
            '<span class="gam-srv-players" title="' + players + ' players"><i class="fa-solid fa-user gam-stat-ico"></i> <strong>' + esc(String(s.player_count)) + '</strong> out of <strong>' + esc(String(s.max_players)) + '</strong></span>' +
            '<span class="gam-srv-avatars">' + window.renderGameServerAvatars(s.players) + '</span>' +
            '</div>' +
            '<span class="gam-srv-btns">' +
            '<button type="button" class="primary hpysuc hpyinl hpysm" onclick="window.joinGame(event)"><span>Join</span></button>' +
            '</span></div>';
    });
    list.innerHTML = html;
    if (window.htmx) window.htmx.process(list);
};

window.rateGame = function(rating, event) {
    if (event) {
        event.preventDefault();
        event.stopPropagation();
    }
    const row = document.getElementById('gamRateRow');
    if (!row || window.ratingGameInFlight) return;
    if (row.dataset.logged !== 'true') {
        window.location.href = '/login';
        return;
    }
    const gameId = row.dataset.gameId;
    if (!gameId) return;
    window.ratingGameInFlight = true;
    const btns = row.querySelectorAll('.gam-ratebtn');
    const msg = document.getElementById('gamRateMsg');
    btns.forEach(function(b) { b.disabled = true; });
    if (msg) { msg.textContent = ''; msg.className = 'shpitm-wearmsg'; }
    const fd = new FormData();
    fd.append('rating', rating);
    fetch('/api/v1/games/' + encodeURIComponent(gameId) + '/rate', {method: 'POST', body: fd, headers: {'HX-Request': 'true'}})
        .then(function(res) { return res.json().then(function(j) { return {ok: res.ok, status: res.status, body: j}; }); })
        .then(function(out) {
            if (!out.ok) {
                if (out.status === 401) {
                    window.location.href = '/login';
                    return;
                }
                throw new Error((out.body && out.body.error) || 'Could not save rating.');
            }
            const d = out.body || {};
            row.dataset.rating = d.user_rating || '';
            btns.forEach(function(b) {
                b.classList.toggle('active-like', b.dataset.rating === 'like' && d.user_rating === '1');
                b.classList.toggle('active-dislike', b.dataset.rating === 'dislike' && d.user_rating === '-1');
                if (b.dataset.rating === rating) {
                    b.classList.add('bump');
                    setTimeout(function() { b.classList.remove('bump'); }, 220);
                }
            });
            updateGamOdometer(document.getElementById('gamLikeCount'), d.likes ?? 0, 18, '');
            updateGamOdometer(document.getElementById('gamDislikeCount'), d.dislikes ?? 0, 18, '');
            const pct = document.getElementById('gamPctVal');
            const pill = document.getElementById('gamPctPill');
            if (pct && pill) {
                if (d.has_rating) {
                    const hue = Math.round((d.like_pct || 0) * 1.2);
                    const col = 'hsl(' + hue + ', 75%, 38%)';
                    updateGamOdometer(pct, d.like_pct, 19, '%');
                    pct.style.color = col;
                    pct.classList.remove('gam-norating');
                    const ico = pill.querySelector('i');
                    if (ico) { ico.style.color = col; ico.classList.remove('gam-norating'); }
                    pill.title = d.likes + ' likes, ' + d.dislikes + ' dislikes';
                    pill.classList.remove('gam-norating');
                } else {
                    pct.textContent = '—';
                    pct.removeAttribute('data-odovalue');
                    pct.style.color = '';
                    pct.classList.add('gam-norating');
                    const ico = pill.querySelector('i');
                    if (ico) { ico.style.color = ''; ico.classList.add('gam-norating'); }
                    pill.title = 'No ratings yet';
                    pill.classList.add('gam-norating');
                }
            }
            const spec = document.getElementById('gamSpecRating');
            if (spec) spec.textContent = d.has_rating ? d.like_pct + '% (' + d.likes + '/' + d.dislikes + ')' : '—';
            window.ratingGameInFlight = false;
            btns.forEach(function(b) { b.disabled = false; });
        })
        .catch(function(err) {
            if (msg) {
                msg.textContent = err.message || 'Could not save rating.';
                msg.className = 'shpitm-wearmsg err';
                setTimeout(function() { if (msg.classList.contains('err')) msg.textContent = ''; }, 4200);
            }
            window.ratingGameInFlight = false;
            btns.forEach(function(b) { b.disabled = false; });
        });
};

const updateGamOdometer = (element, value, digitHeight, suffix) => {
    if (!element) return;
    const valString = String(value);
    element.dataset.odovalue = valString;
    let strips = element.querySelectorAll(':scope > .dgtcnt');
    if (strips.length !== valString.length) {
        let html = '';
        for (let i = 0; i < valString.length; i++) {
            html += '<span class="dgtcnt"><span class="dgtstp">';
            for (let j = 0; j <= 9; j++) html += '<span class="dgtnum">' + j + '</span>';
            html += '</span></span>';
        }
        if (suffix) html += '<span>' + suffix + '</span>';
        element.innerHTML = html;
        strips = element.querySelectorAll(':scope > .dgtcnt');
    }
    for (let i = 0; i < valString.length; i++) {
        const digit = parseInt(valString[i], 10) || 0;
        const strip = strips[i].querySelector('.dgtstp');
        if (strip) strip.style.transform = 'translateY(-' + (digit * digitHeight) + 'px)';
    }
};

const initGamOdometer = (id, digitHeight, suffix) => {
    const el = document.getElementById(id);
    if (!el) return;
    if (el.dataset.odovalue) {
        updateGamOdometer(el, el.dataset.odovalue, digitHeight, suffix);
        return;
    }
    if (el.querySelector(':scope > .dgtcnt')) return;
    const m = (el.textContent || '').match(/(\d+)/);
    if (m) updateGamOdometer(el, parseInt(m[1], 10), digitHeight, suffix);
};

window.showGameBannerSlide = function(index) {
    const track = document.getElementById('gamDetTrack');
    if (!track) return;
    const slides = track.querySelectorAll('.gam-det-slide');
    if (slides.length === 0) return;
    if (index >= slides.length) {
        window.gameBannerIndex = 0;
    } else if (index < 0) {
        window.gameBannerIndex = slides.length - 1;
    } else {
        window.gameBannerIndex = index;
    }
    track.style.transform = 'translateX(-' + (window.gameBannerIndex * 100) + '%)';
};

window.startGameBannerInterval = function() {
    if (window.gameBannerInterval) clearInterval(window.gameBannerInterval);
    if (!document.getElementById('gamDetTrack')) return;
    window.gameBannerInterval = setInterval(function() {
        window.showGameBannerSlide(window.gameBannerIndex + 1);
    }, 5000);
};

window.moveGameBanner = function(direction, event) {
    if (event) {
        event.stopPropagation();
        event.preventDefault();
    }
    window.startGameBannerInterval();
    window.showGameBannerSlide(window.gameBannerIndex + direction);
};

window.triggerWarningsAcceptedKey = function(modal) {
    const gameId = modal.getAttribute('data-game-id') || window.gameDetailGameId();
    if (!gameId) return '';
    return 'triggerWarningsAccepted:' + gameId;
};

window.closeTriggerWarningsModal = function(accepted) {
    const btn = document.getElementById('triggerWarningsContinue');
    if (btn && btn.disabled) return;
    const modal = document.getElementById('triggerWarningsModal');
    if (modal) {
        modal.classList.remove('active');
        if (accepted) {
            const key = window.triggerWarningsAcceptedKey(modal);
            if (key) {
                try {
                    localStorage.setItem(key, modal.getAttribute('data-warnings') || '');
                } catch (e) {}
            }
        }
    }
    if (window.triggerWarningsTimer) {
        clearInterval(window.triggerWarningsTimer);
        window.triggerWarningsTimer = null;
    }
};

window.triggerWarningsGoBack = function() {
    const modal = document.getElementById('triggerWarningsModal');
    if (modal) modal.classList.remove('active');
    if (window.triggerWarningsTimer) {
        clearInterval(window.triggerWarningsTimer);
        window.triggerWarningsTimer = null;
    }
    if (window.history.length > 1) {
        window.history.back();
    } else {
        window.navigateWithHistory('/play');
    }
};

window.initTriggerWarningsModal = function() {
    const modal = document.getElementById('triggerWarningsModal');
    const btn = document.getElementById('triggerWarningsContinue');
    if (!modal || !btn) {
        if (window.triggerWarningsTimer) {
            clearInterval(window.triggerWarningsTimer);
            window.triggerWarningsTimer = null;
        }
        return;
    }
    const key = window.triggerWarningsAcceptedKey(modal);
    let accepted = '';
    if (key) {
        try {
            accepted = localStorage.getItem(key) || '';
        } catch (e) {}
    }
    if (accepted && accepted === (modal.getAttribute('data-warnings') || '')) {
        modal.classList.remove('active');
        return;
    }
    modal.classList.add('active');
    const label = btn.querySelector('span');
    if (window.triggerWarningsTimer) {
        clearInterval(window.triggerWarningsTimer);
        window.triggerWarningsTimer = null;
    }
    let remaining = 5;
    btn.disabled = true;
    if (label) label.textContent = 'Continue (' + remaining + ')';
    window.triggerWarningsTimer = setInterval(function() {
        remaining -= 1;
        if (remaining <= 0) {
            clearInterval(window.triggerWarningsTimer);
            window.triggerWarningsTimer = null;
            btn.disabled = false;
            if (label) label.textContent = 'Continue';
            return;
        }
        if (label) label.textContent = 'Continue (' + remaining + ')';
    }, 1000);
};

const initGameDetailPage = () => {
    window.ratingGameInFlight = false;
    window.joinGameInFlight = false;
    window.gameBannerIndex = 0;
    const aboutBtn = document.querySelector('.gam-det-tabbtn[data-tab="about"]');
    if (aboutBtn) {
        document.querySelectorAll('.gam-det-tabbtn').forEach(function(t) { t.classList.remove('active'); });
        aboutBtn.classList.add('active');
    }
    const aboutPane = document.getElementById('gamDetPaneAbout');
    const serversPane = document.getElementById('gamDetPaneServers');
    if (aboutPane) aboutPane.hidden = false;
    if (serversPane) serversPane.hidden = true;
    const banner = document.getElementById('gamDetBanner');
    if (banner) {
        window.showGameBannerSlide(0);
        window.startGameBannerInterval();
        banner.onmouseenter = function() {
            if (window.gameBannerInterval) clearInterval(window.gameBannerInterval);
        };
        banner.onmouseleave = function() {
            window.startGameBannerInterval();
        };
    } else if (window.gameBannerInterval) {
        clearInterval(window.gameBannerInterval);
    }
    initGamOdometer('gamLikeCount', 18, '');
    initGamOdometer('gamDislikeCount', 18, '');
    initGamOdometer('gamPctVal', 19, '%');
    window.updateGameDetailTabIndicator();
    window.initTriggerWarningsModal();

    document.querySelectorAll('#gamDetRelatedRow .avtldimg, .gam-det-bannerimg').forEach(img => {
        if (img.complete) img.classList.add('ld');
    });

    window.removeEventListener('resize', window.updateGameDetailTabIndicator);
    window.addEventListener('resize', window.updateGameDetailTabIndicator);
};

if (!window.__gamDetNavBound) {
    window.__gamDetNavBound = true;
    document.addEventListener('click', function(event) {
        const card = event.target.closest('#gamDetRelatedRow .shp-card[data-id]');
        if (card && !event.target.closest('a')) {
            const id = card.getAttribute('data-id');
            if (id) {
                window.navigateWithHistory('/play/' + encodeURIComponent(id));
            }
        }
    });
}

if (!window.__gamDetLoadBound) {
    window.__gamDetLoadBound = true;
    document.addEventListener('load', function(event) {
        const img = event.target;
        if (img && img.tagName === 'IMG' && img.classList.contains('avtldimg')) {
            img.classList.add('ld');
        }
    }, true);
}

initGameDetailPage();

document.addEventListener('htmx:afterSettle', initGameDetailPage);
document.addEventListener('htmx:beforeTransition', function() {
    window.removeEventListener('resize', window.updateGameDetailTabIndicator);
    if (window.triggerWarningsTimer) {
        clearInterval(window.triggerWarningsTimer);
        window.triggerWarningsTimer = null;
    }
});

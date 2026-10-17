let currentGamePage = 1;
let currentGameGenre = 'all';
let gameSearchTimer = null;
let loadingGames = false;
let noMoreGames = false;

const gameEsc = function(str) {
    if (window.escapeHTML) return window.escapeHTML(String(str));
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
};

window.setGameGenre = function(genre) {
    currentGameGenre = genre || 'all';

    document.querySelectorAll('#gamGenreBar .gam-pill').forEach(p => {
        p.classList.toggle('active', p.dataset.genre === currentGameGenre);
    });
};

window.isGameBrowsing = function() {
    const barSearch = document.getElementById('playBarSearch');
    const q = barSearch ? barSearch.value.trim() : '';
    const sortEl = document.getElementById('gameSortSelect');
    const sort = sortEl ? sortEl.value : 'popular';
    return q !== '' || currentGameGenre !== 'all' || sort !== 'popular';
};

window.showGameResults = function() {
    const shelves = document.getElementById('gamShelves');
    const search = document.getElementById('gamSearch');
    if (shelves) shelves.style.display = 'none';
    if (search) search.style.display = '';
};

window.showGameShelves = function() {
    const shelves = document.getElementById('gamShelves');
    const search = document.getElementById('gamSearch');
    if (shelves) shelves.style.display = '';
    if (search) search.style.display = 'none';
};

window.refreshGameView = function() {
    if (window.isGameBrowsing()) {
        window.showGameResults();
        window.fetchGames(1, false);
    } else {
        window.showGameShelves();
    }
};

window.updateGameSearchTitle = function(q) {
    const titleEl = document.getElementById('gameSearchTitle');
    if (!titleEl) return;
    if (q) {
        titleEl.textContent = 'Search Results';
        return;
    }
    if (currentGameGenre && currentGameGenre !== 'all') {
        const pill = document.querySelector('#gamGenreBar .gam-pill.active span');
        const name = pill ? pill.textContent.trim() : currentGameGenre;
        titleEl.textContent = name + ' Games';
    } else {
        titleEl.textContent = 'Search Results';
    }
};

window.switchGameGenre = function(genre, event) {
    const clickedPill = event ? event.currentTarget : null;
    if (clickedPill && clickedPill.classList.contains('active')) return;

    window.setGameGenre(genre);
    window.refreshGameView();
};

window.toggleGameShelf = function(btn) {
    if (!btn) return;
    const shelf = btn.closest('.gam-shelf');
    const row = shelf ? shelf.querySelector('.gam-row') : null;
    if (!row) return;
    const expanded = row.classList.toggle('expanded');
    btn.textContent = expanded ? '‹ Show Less' : 'See All ›';
};

window.gameCardHTML = function(game) {
    const safeName = gameEsc(game.name);
    const safeCreator = gameEsc(game.creator_name || 'VERTEXIA');

    let likeHtml = '';
    if (game.like_pct >= 0) {
        const hue = Math.round(game.like_pct * 1.2);
        likeHtml = `<span class="shp-price-val" title="${game.likes} likes, ${game.dislikes} dislikes">
            <i class="fa-solid fa-thumbs-up gam-stat-ico" style="color: hsl(${hue}, 75%, 38%);"></i>
            <span style="color: hsl(${hue}, 75%, 38%);">${game.like_pct}%</span>
        </span>`;
    } else {
        likeHtml = `<span class="shp-price-val gam-norating" title="No ratings yet">
            <i class="fa-solid fa-thumbs-up gam-stat-ico"></i>
            <span>&mdash;</span>
        </span>`;
    }

    return `
        <div class="shp-card" data-id="${game.id}">
            <div class="avtitmimg">
                <img src="/static/uploads/games/${game.id}.png" alt="${safeName}" loading="lazy" class="avtldimg" onerror="this.src='/static/useful/temp/pfp.png'; this.classList.add('ld');" onload="this.classList.add('ld');">
            </div>
            <div class="avtitmmeta">
                <span class="avtitmnm">${safeName}</span>
                <span class="avtitmcr">by ${safeCreator}</span>
            </div>
            <div class="shp-price-row gam-stats">
                ${likeHtml}
                <span class="shp-price-val" title="${game.playing} playing">
                    <i class="fa-solid fa-user gam-stat-ico"></i>
                    <span>${game.playing}</span>
                </span>
            </div>
        </div>
    `;
};

window.checkGameViewportFill = function() {
    if (loadingGames || noMoreGames) return;

    const grid = document.getElementById('gameGrid');
    if (!grid) return;

    const windowHeight = window.innerHeight || document.documentElement.clientHeight;
    const fullHeight = Math.max(
        document.body.scrollHeight,
        document.body.offsetHeight,
        document.documentElement.clientHeight,
        document.documentElement.scrollHeight,
        document.documentElement.offsetHeight
    );

    if (fullHeight <= windowHeight + 150) {
        window.fetchGames(currentGamePage + 1, true);
    }
};

window.fetchGames = function(page = 1, append = false) {
    if (loadingGames) return;
    if (append && noMoreGames) return;

    const grid = document.getElementById('gameGrid');
    if (!grid) return;

    loadingGames = true;

    const spinnerHtml = '<div id="gameScrollSpinner" class="musld" style="grid-column: 1 / -1;"><svg class="dmspn" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">' +
        '<rect class="sq sq-1" x="43" y="7" width="14" height="14"/>' +
        '<rect class="sq sq-2" x="25" y="25" width="14" height="14"/>' +
        '<rect class="sq sq-3" x="7" y="43" width="14" height="14"/>' +
        '<rect class="sq sq-4" x="25" y="61" width="14" height="14"/>' +
        '<rect class="sq sq-5" x="43" y="79" width="14" height="14"/>' +
        '<rect class="sq sq-6" x="61" y="61" width="14" height="14"/>' +
        '<rect class="sq sq-7" x="79" y="43" width="14" height="14"/>' +
        '<rect class="sq sq-8" x="61" y="25" width="14" height="14"/>' +
        '</svg></div>';

    if (!append) {
        currentGamePage = 1;
        noMoreGames = false;
        grid.innerHTML = spinnerHtml;
    } else {
        const existingSpinner = document.getElementById('gameScrollSpinner');
        if (!existingSpinner) {
            grid.insertAdjacentHTML('beforeend', spinnerHtml);
        }
    }

    const barSearch = document.getElementById('playBarSearch');
    const q = (barSearch && barSearch.value.trim()) || '';

    if (!append) {
        window.updateGameSearchTitle(q);
    }

    const sortEl = document.getElementById('gameSortSelect');
    const sort = sortEl ? sortEl.value : 'popular';

    const params = new URLSearchParams({
        genre: currentGameGenre,
        q: q,
        sort: sort,
        page: page,
        limit: 20
    });

    fetch('/api/v1/games?' + params.toString())
        .then(res => res.json())
        .then(data => {
            const spinner = document.getElementById('gameScrollSpinner');
            if (spinner) spinner.remove();

            if (!data || data.error) {
                if (!append) {
                    grid.innerHTML = '<div class="feedemt" style="grid-column: 1 / -1;">Failed to load games.</div>';
                }
                loadingGames = false;
                return;
            }

            const totalPages = Math.max(1, data.total_pages || 1);

            const countEl = document.getElementById('gameResultCount');
            if (countEl) {
                const total = data.total || 0;
                countEl.textContent = total + (total === 1 ? ' game' : ' games');
            }

            if (!data.games || data.games.length === 0) {
                if (!append) {
                    grid.innerHTML = '<div class="feedemt" style="grid-column: 1 / -1;">No games found matching your filters.</div>';
                }
                noMoreGames = true;
            } else {
                const cardsHtml = data.games.map(game => window.gameCardHTML(game)).join('');
                if (!append) {
                    grid.innerHTML = cardsHtml;
                } else {
                    grid.insertAdjacentHTML('beforeend', cardsHtml);
                }

                document.querySelectorAll('#gameGrid .avtldimg').forEach(img => {
                    if (img.complete) {
                        img.classList.add('ld');
                    } else {
                        img.addEventListener('load', () => {
                            img.classList.add('ld');
                            window.checkGameViewportFill();
                        });
                        img.addEventListener('error', () => {
                            img.classList.add('ld');
                        });
                    }
                });

                currentGamePage = page;
                if (page >= totalPages) {
                    noMoreGames = true;
                }
            }

            loadingGames = false;
            setTimeout(() => {
                window.checkGameViewportFill();
            }, 100);

            try {
                const url = new URL(window.location.href);
                if (q) url.searchParams.set('q', q); else url.searchParams.delete('q');
                if (currentGameGenre !== 'all') url.searchParams.set('genre', currentGameGenre); else url.searchParams.delete('genre');
                if (sort !== 'popular') url.searchParams.set('sort', sort); else url.searchParams.delete('sort');
                if (page > 1) url.searchParams.set('page', page); else url.searchParams.delete('page');
                window.history.replaceState({}, document.title, url.search ? url.pathname + url.search : url.pathname);
            } catch (e) {}
        })
        .catch(() => {
            const spinner = document.getElementById('gameScrollSpinner');
            if (spinner) spinner.remove();
            if (!append) {
                grid.innerHTML = '<div class="feedemt" style="grid-column: 1 / -1;">Error loading games.</div>';
            }
            loadingGames = false;
        });
};

window.handleGameScroll = function() {
    if (loadingGames || noMoreGames) return;

    const grid = document.getElementById('gameGrid');
    if (!grid) return;

    const scrollTop = window.scrollY || window.pageYOffset || document.documentElement.scrollTop || 0;
    const windowHeight = window.innerHeight || document.documentElement.clientHeight;
    const fullHeight = Math.max(
        document.body.scrollHeight,
        document.body.offsetHeight,
        document.documentElement.clientHeight,
        document.documentElement.scrollHeight,
        document.documentElement.offsetHeight
    );

    if (scrollTop + windowHeight >= fullHeight - 350) {
        window.fetchGames(currentGamePage + 1, true);
    }
};

const initGamePage = () => {
    const activePill = document.querySelector('#gamGenreBar .gam-pill.active');
    if (activePill && activePill.dataset.genre) {
        currentGameGenre = activePill.dataset.genre;
    }
    window.setGameGenre(currentGameGenre);

    const barSearch = document.getElementById('playBarSearch');

    const handleSearchInput = function() {
        if (gameSearchTimer) clearTimeout(gameSearchTimer);
        gameSearchTimer = setTimeout(() => {
            window.refreshGameView();
        }, 300);
    };

    if (barSearch) barSearch.oninput = handleSearchInput;

    const sortInput = document.getElementById('gameSortSelect');
    if (sortInput) {
        sortInput.onchange = function() {
            window.refreshGameView();
        };
    }

    const markLoaded = function(event) {
        const img = event.target;
        if (img && img.tagName === 'IMG' && img.classList.contains('avtldimg')) {
            img.classList.add('ld');
        }
    };
    if (!window.__gamLoadBound) {
        window.__gamLoadBound = true;
        document.addEventListener('load', markLoaded, true);
        document.addEventListener('error', markLoaded, true);
    }

    document.querySelectorAll('#gameGrid .avtldimg, .gam-row .avtldimg').forEach(img => {
        if (img.complete) img.classList.add('ld');
    });

    window.removeEventListener('scroll', window.handleGameScroll);
    window.addEventListener('scroll', window.handleGameScroll);
    window.removeEventListener('resize', window.checkGameViewportFill);
    window.addEventListener('resize', window.checkGameViewportFill);
};

if (!window.__gamCardNavBound) {
    window.__gamCardNavBound = true;
    document.addEventListener('click', function(event) {
        var card = event.target.closest('#gameGrid .shp-card[data-id], .gam-row .shp-card[data-id]');
        if (card && !event.target.closest('button') && !event.target.closest('a')) {
            var id = card.getAttribute('data-id');
            if (id) {
                window.navigateWithHistory('/play/' + encodeURIComponent(id));
            }
        }
    });
}

initGamePage();

document.addEventListener('htmx:afterSettle', initGamePage);
document.addEventListener('htmx:beforeTransition', function() {
    window.removeEventListener('scroll', window.handleGameScroll);
    window.removeEventListener('resize', window.checkGameViewportFill);
});

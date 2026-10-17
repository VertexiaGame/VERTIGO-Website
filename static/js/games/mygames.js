window.viewGame = function(id) {
    if (!id) return;
    window.navigateWithHistory('/play/' + encodeURIComponent(id));
};

window.editGame = function(id) {
    if (!id) return;
    window.navigateWithHistory('/create/games/' + encodeURIComponent(id) + '/edit');
};

window.buildStudioEditUri = function(id) {
    return 'vertexia-studio://edit?game=' + encodeURIComponent(id) + '&base=' + encodeURIComponent(window.location.origin);
};

window.editInStudio = function(id) {
    if (!id) return;
    window.location.href = window.buildStudioEditUri(id);
};

const initMyGamesPage = () => {
    document.querySelectorAll('#myGamesGrid img.avtldimg').forEach(function(img) {
        if (img.complete && img.naturalWidth > 0) img.classList.add('ld');
    });
    const searchInput = document.getElementById('myGamesSearch');
    if (searchInput) {
        searchInput.oninput = function() {
            const q = this.value.trim().toLowerCase();
            document.querySelectorAll('#myGamesGrid .shp-card').forEach(function(card) {
                const nameEl = card.querySelector('.avtitmnm');
                const match = !q || (nameEl && nameEl.textContent.toLowerCase().indexOf(q) !== -1);
                card.style.display = match ? '' : 'none';
            });
        };
    }
};

if (!window.__myGamesNavBound) {
    window.__myGamesNavBound = true;
    document.addEventListener('click', function(event) {
        const editBtn = event.target.closest ? event.target.closest('[data-edit-game]') : null;
        if (editBtn) {
            event.preventDefault();
            event.stopPropagation();
            window.editGame(editBtn.getAttribute('data-edit-game'));
            return;
        }
        const studioBtn = event.target.closest ? event.target.closest('[data-edit-studio]') : null;
        if (studioBtn) {
            event.preventDefault();
            event.stopPropagation();
            window.editInStudio(studioBtn.getAttribute('data-edit-studio'));
            return;
        }
        const card = event.target.closest ? event.target.closest('#myGamesGrid .shp-card[data-goto="game"]') : null;
        if (card && !event.target.closest('button') && !event.target.closest('a')) {
            window.viewGame(card.getAttribute('data-id'));
        }
    });

    document.addEventListener('keydown', function(event) {
        if (event.key !== 'Enter' && event.key !== ' ') return;
        const card = event.target.closest ? event.target.closest('#myGamesGrid .shp-card[data-goto="game"]') : null;
        if (card && !event.target.closest('button')) {
            event.preventDefault();
            window.viewGame(card.getAttribute('data-id'));
        }
    });
}

initMyGamesPage();
document.addEventListener('htmx:afterSettle', initMyGamesPage);

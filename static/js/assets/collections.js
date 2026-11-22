(function() {
    const searchIDs = { assets: 'assetSearch', items: 'itemSearch', games: 'myGamesSearch', clans: 'myClansSearch' };

    function updateCollectionTabIndicator() {
        const tabs = document.querySelector('.collection-tabs');
        if (!tabs) return;
        const indicator = tabs.querySelector('.tabind');
        const active = tabs.querySelector('.tabbtn.active');
        if (!indicator || !active) return;
        indicator.style.left = active.offsetLeft + 'px';
        indicator.style.top = active.offsetTop + 'px';
        indicator.style.width = active.offsetWidth + 'px';
        indicator.style.height = active.offsetHeight + 'px';
    }

    window.switchCollectionTab = function(event, tab) {
        if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return true;
        const panel = document.getElementById('collection-' + tab);
        if (!panel) return true;
        event.preventDefault();
        const selected = event.currentTarget;
        if (selected.classList.contains('active')) return false;

        document.querySelectorAll('[data-collection-tab]').forEach(function(link) {
            const active = link === selected;
            link.classList.toggle('active', active);
            if (active) link.setAttribute('aria-current', 'page');
            else link.removeAttribute('aria-current');
        });
        document.querySelectorAll('.settabvwp > .settabcntnt').forEach(function(content) {
            content.classList.toggle('active', content === panel);
        });
        Object.keys(searchIDs).forEach(function(name) {
            const input = document.getElementById(searchIDs[name]);
            if (input) input.hidden = name !== tab;
        });
        document.querySelectorAll('[data-collection-create]').forEach(function(link) {
            link.hidden = link.dataset.collectionCreate !== tab;
        });
        if (tab !== 'assets' && window.assetAudio) window.assetAudio.pause();
        updateCollectionTabIndicator();
        window.history.replaceState(window.history.state, '', selected.href);
        return false;
    };

    updateCollectionTabIndicator();
    if (!window.__collectionTabsBound) {
        window.__collectionTabsBound = true;
        document.addEventListener('htmx:afterSettle', updateCollectionTabIndicator);
        window.addEventListener('resize', updateCollectionTabIndicator);
    }
})();

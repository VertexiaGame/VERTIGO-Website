window.filterFollowUsers = function(input) {
    const query = input.value.trim().toLocaleLowerCase();
    document.querySelectorAll('[id^="follow-tab-"]').forEach(panel => {
        const cards = panel.querySelectorAll('[data-follow-user]');
        let visible = 0;
        cards.forEach(card => {
            const matches = card.textContent.toLocaleLowerCase().includes(query);
            card.hidden = !matches;
            if (matches) visible++;
        });
        const empty = panel.querySelector('[data-follow-search-empty]');
        if (empty) empty.hidden = visible !== 0;
    });
    if (window.updateFriendsViewportHeight) window.updateFriendsViewportHeight();
};

window.switchFollowTab = function(event, tab) {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return true;
    const target = document.getElementById('follow-tab-' + tab);
    if (!target) return true;
    event.preventDefault();
    document.querySelectorAll('.follow-tabs .tabbtn').forEach(button => {
        button.classList.toggle('active', button === event.currentTarget);
    });
    document.querySelectorAll('[id^="follow-tab-"]').forEach(panel => {
        panel.classList.toggle('active', panel === target);
    });
    if (window.updateFriendsTabIndicator) window.updateFriendsTabIndicator();
    if (window.updateFriendsViewportHeight) window.updateFriendsViewportHeight();
    const url = new URL(window.location.href);
    url.searchParams.set('tab', tab);
    window.history.replaceState(window.history.state, '', url);
    return false;
};

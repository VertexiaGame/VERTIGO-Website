(function() {
    const applyClanSearch = function() {
        const input = document.getElementById('myClansSearch');
        if (!input) return;

        const q = input.value.trim().toLowerCase();
        let shown = 0;

        document.querySelectorAll('#myClansGrid .cln-mine-card').forEach(function(card) {
            const name = (card.dataset.clanName || '').toLowerCase();
            const match = !q || name.indexOf(q) !== -1;
            card.hidden = !match;
            if (match) shown++;
        });

        const empty = document.getElementById('myClansEmpty');
        if (empty) empty.hidden = !(q && shown === 0);
    };

    const initMyClans = function() {
        const page = document.getElementById('myClansPage');
        if (!page) return;

        const input = document.getElementById('myClansSearch');
        if (input && input.dataset.clnBound !== '1') {
            input.dataset.clnBound = '1';
            input.addEventListener('input', applyClanSearch);
        }
        if (input && input.value) applyClanSearch();
    };

    initMyClans();
    document.addEventListener('htmx:afterSettle', initMyClans);
})();

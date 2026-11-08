(function () {
    var filterForumBoards = function (rawQuery) {
        var input = document.getElementById('fbdSearchInput');
        var list = document.getElementById('fbdList');
        if (!list) return;

        var q = (rawQuery !== undefined ? rawQuery : (input ? input.value : '')).trim().toLowerCase();
        var shown = 0;
        var cards = list.querySelectorAll('.fbd-card');

        cards.forEach(function (card) {
            var name = (card.getAttribute('data-board-name') || '').toLowerCase();
            var desc = (card.getAttribute('data-board-desc') || '').toLowerCase();
            var match = !q || name.indexOf(q) !== -1 || desc.indexOf(q) !== -1;
            var wasHidden = card.hidden;
            card.hidden = !match;
            if (match) {
                shown++;
                if (wasHidden) {
                    card.classList.remove('is-filtered-in');
                    void card.offsetWidth;
                    card.classList.add('is-filtered-in');
                }
            } else {
                card.classList.remove('is-filtered-in');
            }
        });

        var empty = document.getElementById('fbdSearchEmpty');
        if (empty) empty.hidden = !(q && shown === 0);
    };

    var filterForumThreads = function (rawQuery) {
        var input = document.getElementById('fthSearchInput');
        var list = document.getElementById('fthList');
        if (!list) return;

        var q = (rawQuery !== undefined ? rawQuery : (input ? input.value : '')).trim().toLowerCase();
        var shown = 0;
        var rows = list.querySelectorAll('.fth-row');

        rows.forEach(function (row) {
            var title = (row.getAttribute('data-thread-title') || '').toLowerCase();
            var author = (row.getAttribute('data-thread-author') || '').toLowerCase();
            var match = !q || title.indexOf(q) !== -1 || author.indexOf(q) !== -1;
            var wasHidden = row.hidden;
            row.hidden = !match;
            if (match) {
                shown++;
                if (wasHidden) {
                    row.classList.remove('is-filtered-in');
                    void row.offsetWidth;
                    row.classList.add('is-filtered-in');
                }
            } else {
                row.classList.remove('is-filtered-in');
            }
        });

        var table = document.getElementById('fthTable');
        var empty = document.getElementById('fthSearchEmpty');
        var noneFound = !!(q && shown === 0);
        if (table) table.hidden = noneFound;
        if (empty) empty.hidden = !noneFound;
    };

    var toggleComposeDetails = function (details, summary) {
        if (!details || !summary || typeof details.animate !== 'function') {
            if (details) details.open = !details.open;
            return;
        }
        if (details._composeAnim) {
            details._composeAnim.cancel();
            details._composeAnim = null;
        }

        if (!details.open) {
            var startH = details.offsetHeight;
            details.classList.remove('is-closing');
            details.open = true;
            var endH = details.offsetHeight;
            var anim = details.animate(
                [{ height: startH + 'px' }, { height: endH + 'px' }],
                { duration: 260, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' }
            );
            details._composeAnim = anim;
            anim.onfinish = anim.oncancel = function () {
                details.style.height = '';
                details._composeAnim = null;
            };
            var firstInput = details.querySelector('input[type="text"], textarea');
            if (firstInput) {
                setTimeout(function () {
                    firstInput.focus({ preventScroll: true });
                }, 80);
            }
        } else {
            var openH = details.offsetHeight;
            var closedH = summary.offsetHeight + 2;
            details.classList.add('is-closing');
            var closeAnim = details.animate(
                [{ height: openH + 'px' }, { height: closedH + 'px' }],
                { duration: 210, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' }
            );
            details._composeAnim = closeAnim;
            closeAnim.onfinish = function () {
                details.open = false;
                details.classList.remove('is-closing');
                details.style.height = '';
                details._composeAnim = null;
            };
            closeAnim.oncancel = function () {
                details.classList.remove('is-closing');
                details.style.height = '';
                details._composeAnim = null;
            };
        }
    };

    var initForum = function () {
        var boardInput = document.getElementById('fbdSearchInput');
        if (boardInput && boardInput.dataset.fbdBound !== '1') {
            boardInput.dataset.fbdBound = '1';
            boardInput.addEventListener('input', function () {
                filterForumBoards(boardInput.value);
            });
        }
        if (boardInput && boardInput.value) filterForumBoards(boardInput.value);

        var threadInput = document.getElementById('fthSearchInput');
        if (threadInput && threadInput.dataset.fbdBound !== '1') {
            threadInput.dataset.fbdBound = '1';
            threadInput.addEventListener('input', function () {
                filterForumThreads(threadInput.value);
            });
        }
        if (threadInput && threadInput.value) filterForumThreads(threadInput.value);

        var barInput = document.getElementById('forumBarSearch');
        if (barInput && barInput.dataset.fbdBound !== '1') {
            barInput.dataset.fbdBound = '1';
            barInput.addEventListener('input', function () {
                if (document.getElementById('fthList')) {
                    if (threadInput) threadInput.value = barInput.value;
                    filterForumThreads(barInput.value);
                } else if (document.getElementById('fbdList')) {
                    if (boardInput) boardInput.value = barInput.value;
                    filterForumBoards(barInput.value);
                }
            });
        }
    };

    initForum();

    if (!window.__fbdBound) {
        window.__fbdBound = true;

        document.addEventListener('click', function (e) {
            if (!e.target || !e.target.closest) return;

            var rulesBtn = e.target.closest('[data-fbd-rules]');
            if (rulesBtn) {
                var box = rulesBtn.closest('.fbd-rules');
                if (box) {
                    var open = box.classList.toggle('is-open');
                    rulesBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
                }
            }
        });

        document.addEventListener('htmx:afterSettle', initForum);
    }

    if (!window.__clnComposeAnimBound) {
        window.__clnComposeAnimBound = true;

        document.addEventListener('click', function (e) {
            if (!e.target || !e.target.closest) return;
            var summary = e.target.closest('details.cln-compose > summary.cln-compose-sum');
            if (!summary) return;
            var details = summary.parentElement;
            if (!details || details.tagName !== 'DETAILS') return;
            if (details.closest('.forum-detail')) return;
            e.preventDefault();
            toggleComposeDetails(details, summary);
        });
    }
})();

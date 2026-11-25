import { initImageGalleries, initImageUploads } from '../image-attachments.js';

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
        var rows = list.querySelectorAll('.forum-thread-item');

        rows.forEach(function (row) {
            var title = (row.getAttribute('data-thread-title') || '').toLowerCase();
            var author = (row.getAttribute('data-thread-author') || '').toLowerCase();
            var match = !q || title.indexOf(q) !== -1 || author.indexOf(q) !== -1;
            var wasHidden = row.hidden;
            row.hidden = !match;
            if (match) {
                shown++;
                if (wasHidden) {
                    row.style.animation = 'none';
                    row.classList.add('is-filtered-in');
                    void row.offsetWidth;
                    row.style.animation = '';
                }
            }
        });

        var table = document.getElementById('fthTable');
        var empty = document.getElementById('fthSearchEmpty');
        var noneFound = !!(q && shown === 0);
        if (table) table.hidden = noneFound;
        if (empty) empty.hidden = !noneFound;
    };

    var sortForumThreads = async function (link) {
        var results = document.getElementById('fthResults');
        var error = document.getElementById('fthSortError');
        if (!results) { window.location.href = link.href; return; }
        if (window.forumSortController) window.forumSortController.abort();
        var controller = new AbortController();
        window.forumSortController = controller;
        results.classList.remove('is-sorting');
        results.setAttribute('aria-busy', 'true');
        if (error) { error.hidden = true; error.classList.remove('visible'); }
        try {
            var response = await fetch(link.href, {signal: controller.signal});
            if (!response.ok) throw new Error('Could not sort threads. Please try again.');
            var doc = new DOMParser().parseFromString(await response.text(), 'text/html');
            var replacement = doc.getElementById('fthResults');
            if (!replacement) throw new Error('Could not sort threads. Please try again.');
            if (!results.isConnected || controller.signal.aborted) return;
            results.classList.add('is-sorting');
            var reduce = document.documentElement.classList.contains('reduced-motion') || (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
            if (!reduce) await new Promise(function (resolve) { setTimeout(resolve, 160); });
            if (!results.isConnected || controller.signal.aborted) return;
            results.innerHTML = replacement.innerHTML;
            if (window.htmx) window.htmx.process(results);
            var input = document.getElementById('fthSearchInput');
            if (input && input.value) filterForumThreads(input.value);
            document.querySelectorAll('#forumBoardPane .fbd-sort-btn').forEach(function (item) {
                var active = item === link;
                item.classList.toggle('is-active', active);
                if (active) item.setAttribute('aria-current', 'true'); else item.removeAttribute('aria-current');
            });
            window.history.replaceState(window.history.state, '', link.href);
            void results.offsetWidth;
            results.classList.remove('is-sorting');
        } catch (err) {
            if (err.name !== 'AbortError' && error && results.isConnected) {
                error.textContent = 'Could not sort threads. Please try again.';
                error.hidden = false; error.classList.add('visible');
            }
        } finally {
            if (window.forumSortController === controller) {
                results.classList.remove('is-sorting');
                results.setAttribute('aria-busy', 'false');
                window.forumSortController = null;
            }
        }
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

    var prefersReducedMotion = function () {
        return document.documentElement.classList.contains('reduced-motion') || (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    };

    var clearForumQuote = function (form) {
        if (!form) return;
        var input = form.querySelector('input[name="quote_id"]');
        var draft = form.querySelector('#forumReplyQuote');
        if (input) input.value = '';
        if (draft) draft.hidden = true;
        document.querySelectorAll('#forumThreadPane .forum-post-quote.is-active').forEach(function (btn) {
            btn.classList.remove('is-active');
        });
    };

    var startForumQuote = function (button) {
        var form = document.getElementById('ftdReplyForm');
        var details = form ? form.closest('details.cln-compose') : null;
        var input = form ? form.querySelector('input[name="quote_id"]') : null;
        var draft = form ? form.querySelector('#forumReplyQuote') : null;
        if (!form || !details || !input || !draft) return;

        input.value = button.dataset.forumQuote || '0';
        var author = draft.querySelector('[data-quote-author]');
        var excerpt = draft.querySelector('[data-quote-excerpt]');
        var avatar = draft.querySelector('.forum-quote-avatar');
        if (author) author.textContent = button.dataset.quoteAuthor || 'Player';
        if (excerpt) excerpt.textContent = button.dataset.quoteExcerpt || '';
        if (avatar) avatar.src = button.dataset.quoteHeadshot || '/static/useful/temp/pfp.png';
        draft.hidden = false;
        draft.style.animation = 'none';
        void draft.offsetWidth;
        draft.style.animation = '';

        document.querySelectorAll('#forumThreadPane .forum-post-quote.is-active').forEach(function (btn) {
            btn.classList.remove('is-active');
        });
        button.classList.add('is-active');

        var field = form.querySelector('textarea[name="body"]');
        if (!details.open) {
            var summary = details.querySelector('summary.cln-compose-sum');
            toggleComposeDetails(details, summary);
        } else if (field) {
            field.focus({ preventScroll: true });
        }
        details.scrollIntoView({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'nearest' });
    };

    var jumpToForumQuote = function (link) {
        var target = document.getElementById(link.dataset.forumQuoteJump || '');
        if (!target) return false;
        target.scrollIntoView({ behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'center' });
        target.classList.remove('is-quoted');
        void target.offsetWidth;
        target.classList.add('is-quoted');
        window.history.replaceState(window.history.state, '', '#' + target.id);
        return true;
    };

    var submitForumReply = async function (form) {
        if (form.dataset.submitting === '1') return;
        var field = form.querySelector('textarea[name="body"]');
        var button = form.querySelector('button[type="submit"]');
        var uploads = form.querySelectorAll('[data-image-upload-field] button:not(:disabled), [data-forum-upload]');
        var status = document.getElementById('forumReplyStatus');
        if (!field || !button || !form.reportValidity()) return;
        form.dataset.submitting = '1';
        var body = new FormData(form);
        button.disabled = true; field.disabled = true;
        uploads.forEach(function (control) { control.disabled = true; });
        if (status) { status.hidden = true; status.classList.remove('visible'); }
        var posted = false;
        try {
            var response = await fetch(form.action, {method: 'POST', body: body, headers: {'Accept': 'application/json'}});
            var data = await response.json().catch(function () { return {}; });
            if (!response.ok || !data.success) throw new Error(data.error || 'Could not post your reply. Please try again.');
            posted = true;
            form.reset();
            clearForumQuote(form);
            if (!form.isConnected) return;
            var destination = new URL(data.path || window.location.href, window.location.href);
            if (destination.origin !== window.location.origin) throw new Error('Could not display your reply.');
            var page = await fetch(destination.pathname + destination.search);
            if (!page.ok) throw new Error('Could not display your reply.');
            var doc = new DOMParser().parseFromString(await page.text(), 'text/html');
            var discussion = document.getElementById('forumDiscussion');
            var updated = doc.getElementById('forumDiscussion');
            if (!discussion || !updated || !form.isConnected) throw new Error('Could not display your reply.');
            discussion.innerHTML = updated.innerHTML;
            initImageGalleries(discussion);
            var imageStatus = document.getElementById('forumImageStatus');
            var updatedImageStatus = doc.getElementById('forumImageStatus');
            if (imageStatus && updatedImageStatus) {
                imageStatus.textContent = updatedImageStatus.textContent;
                imageStatus.classList.toggle('visible', updatedImageStatus.classList.contains('visible'));
            }
            var footer = document.querySelector('#thread .forum-post-footer');
            var updatedFooter = doc.querySelector('#thread .forum-post-footer');
            if (footer && updatedFooter) footer.innerHTML = updatedFooter.innerHTML;
            if (window.htmx) window.htmx.process(discussion);
            window.history.replaceState(window.history.state, '', destination.pathname + destination.search + destination.hash);
            var newReply = document.getElementById('reply-' + data.id);
            if (newReply) {
                newReply.classList.add('is-new');
                newReply.scrollIntoView({behavior: prefersReducedMotion() ? 'auto' : 'smooth', block: 'nearest'});
            }
        } catch (err) {
            if (status && form.isConnected) {
                status.textContent = posted ? 'Your reply was posted. Reload the page to see it.' : (err.message || 'Could not post your reply. Please try again.');
                status.hidden = false; status.classList.add('visible');
            }
        } finally {
            delete form.dataset.submitting;
            button.disabled = false; field.disabled = false;
            uploads.forEach(function (control) { control.disabled = false; });
        }
    };

    var initForum = function () {
        initImageGalleries(document);
        initImageUploads(document);
        var replyForm = document.getElementById('ftdReplyForm');
        if (replyForm && replyForm.dataset.forumBound !== '1') {
            replyForm.dataset.forumBound = '1';
            replyForm.addEventListener('submit', function (event) {
                event.preventDefault();
                submitForumReply(replyForm);
            });
        }
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

            var sortLink = e.target.closest('#forumBoardPane .fbd-sort-btn');
            if (sortLink && e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey) {
                e.preventDefault();
                sortForumThreads(sortLink);
                return;
            }

            var quoteBtn = e.target.closest('#forumThreadPane [data-forum-quote]');
            if (quoteBtn) {
                e.preventDefault();
                startForumQuote(quoteBtn);
                return;
            }

            var quoteClear = e.target.closest('#forumThreadPane [data-forum-quote-clear]');
            if (quoteClear) {
                e.preventDefault();
                clearForumQuote(quoteClear.closest('form'));
                return;
            }

            var quoteJump = e.target.closest('#forumThreadPane [data-forum-quote-jump]');
            if (quoteJump && e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey
                && quoteJump.getAttribute('href').charAt(0) === '#') {
                if (jumpToForumQuote(quoteJump)) e.preventDefault();
                return;
            }

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

var moveForumCarousel = function (gallery, direction, announce) {
    var slides = gallery.querySelectorAll('[data-forum-slide]');
    var index = (Number(gallery.dataset.forumIndex) + direction + slides.length) % slides.length;
    gallery.dataset.forumIndex = index;
    gallery.querySelector('[data-forum-track]').style.transform = 'translateX(' + (-index * 100) + '%)';
    slides.forEach(function (slide, slideIndex) {
        slide.inert = slideIndex !== index;
    });
    var position = gallery.querySelector('[data-forum-position]');
    position.setAttribute('aria-live', announce ? 'polite' : 'off');
    position.textContent = (index + 1) + ' / ' + slides.length;
};

var bindForumAutoplay = function (gallery) {
    var toggle = gallery.querySelector('[data-forum-toggle]');
    toggle.hidden = true;
    if (gallery.closest('[data-forum-autoplay="off"]')) return;

    var motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    if (motion.matches || document.documentElement.classList.contains('reduced-motion')) return;

    var paused = gallery.dataset.forumPaused === 'true';
    var timer;
    var updateToggle = function () {
        toggle.setAttribute('aria-label', paused ? 'Play slideshow' : 'Pause slideshow');
        toggle.firstElementChild.classList.toggle('fa-play', paused);
        toggle.firstElementChild.classList.toggle('fa-pause', !paused);
    };
    var start = function () {
        timer = window.setInterval(function () {
            if (!gallery.isConnected) {
                window.clearInterval(timer);
                return;
            }
            var bounds = gallery.getBoundingClientRect();
            if (document.hidden || motion.matches || gallery.matches(':hover, :focus-within') ||
                bounds.bottom <= 0 || bounds.top >= window.innerHeight) return;
            moveForumCarousel(gallery, 1, false);
        }, 5000);
    };
    toggle.addEventListener('click', function () {
        paused = !paused;
        gallery.dataset.forumPaused = paused;
        window.clearInterval(timer);
        if (!paused) start();
        updateToggle();
    });
    toggle.hidden = false;
    updateToggle();
    if (!paused) start();
};

export function initImageGalleries(root) {
    root.querySelectorAll('[data-forum-gallery]').forEach(function (gallery) {
        var controls = gallery.querySelector('.forum-gallery-controls');
        if (!controls || gallery.forumCarouselBound) return;
        gallery.forumCarouselBound = true;
        gallery.dataset.forumIndex = gallery.dataset.forumIndex || 0;
        moveForumCarousel(gallery, 0, false);
        gallery.classList.add('is-carousel');
        controls.hidden = false;
        controls.addEventListener('click', function (event) {
            var button = event.target.closest('[data-forum-step]');
            if (button) moveForumCarousel(gallery, Number(button.dataset.forumStep), true);
        });
        bindForumAutoplay(gallery);
    });
}

let openLibrary;

function bindImageLibrary(field, picker, input, saved, toggleImage) {
    const panel = field.querySelector('[data-image-library]');
    const scroll = panel.querySelector('[data-image-library-scroll]');
    const list = panel.querySelector('[data-image-library-list]');
    const end = panel.querySelector('[data-image-library-end]');
    const status = panel.querySelector('[data-image-library-status]');
    const retry = panel.querySelector('[data-image-library-retry]');
    const cardTemplate = panel.querySelector('[data-image-library-card]');
    const upload = panel.querySelector('[data-image-upload-new]');
    const anchor = picker.querySelector('[data-image-picker-anchor]');
    let session;
    let loading = false;
    let nextPage = '';

    function makeCard(image, index) {
        const name = image.name || 'Image #' + image.id;
        const card = cardTemplate.content.firstElementChild.cloneNode(true);
        const preview = card.querySelector('img');
        card.dataset.savedImage = image.id;
        card.style.setProperty('--image-card-delay', Math.min(index + 1, 8) * 20 + 'ms');
        card.title = name;
        card.querySelector('.image-picker-name').textContent = name;
        preview.addEventListener('load', () => preview.classList.add('is-loaded'), { once: true });
        preview.src = '/api/v1/images/' + encodeURIComponent(image.id);

        if (!image.approved) {
            card.disabled = true;
            card.querySelector('.image-picker-pending').hidden = false;
        } else {
            card.setAttribute('aria-pressed', saved.has(image.id));
            card.addEventListener('click', () => {
                if (!picker.disabled && !input.disabled) toggleImage(image.id, name);
            });
        }
        return card;
    }

    async function loadImages() {
        if (!session || loading) return;
        const { signal } = session;
        loading = true;
        observer.unobserve(end);
        retry.hidden = true;
        list.setAttribute('aria-busy', 'true');
        status.textContent = 'Loading images…';
        try {
            const url = '/api/v1/images' + (nextPage ? '?before=' + encodeURIComponent(nextPage) : '');
            const response = await fetch(url, { signal });
            if (!response.ok) throw new Error('Image library request failed');
            const page = await response.json();
            if (signal.aborted) return;
            list.append(...page.images.map(makeCard));
            nextPage = page.next;
            status.textContent = list.childElementCount ? '' : 'Your uploaded images will appear here.';
        } catch {
            if (signal.aborted) return;
            status.textContent = 'Could not load your images.';
            retry.hidden = false;
        } finally {
            if (!signal.aborted) {
                loading = false;
                list.removeAttribute('aria-busy');
                if (nextPage && retry.hidden) observer.observe(end);
            }
        }
    }

    const observer = new IntersectionObserver(entries => {
        if (retry.hidden && entries.some(entry => entry.isIntersecting)) loadImages();
    }, { root: scroll, rootMargin: '0px 0px 100px 0px' });

    function positionPicker() {
        const viewport = window.visualViewport;
        const viewportLeft = viewport?.offsetLeft ?? 0;
        const viewportTop = viewport?.offsetTop ?? 0;
        const viewportWidth = viewport?.width ?? document.documentElement.clientWidth;
        const viewportHeight = viewport?.height ?? document.documentElement.clientHeight;
        const bounds = picker.getBoundingClientRect();
        const edge = 12;
        const gap = 9;
        const viewportBottom = viewportTop + viewportHeight;
        if (!bounds.height || bounds.bottom <= viewportTop || bounds.top >= viewportBottom) {
            closePicker();
            return;
        }

        const fieldBounds = field.getBoundingClientRect();
        const icon = anchor.getBoundingClientRect();
        const anchorX = icon.left + icon.width / 2;
        const width = Math.min(480, fieldBounds.width, viewportWidth - edge * 2);
        const preferredLeft = field === input.form ? fieldBounds.right - width : fieldBounds.left;
        const left = Math.max(viewportLeft + edge, Math.min(preferredLeft, viewportLeft + viewportWidth - edge - width));
        const below = viewportBottom - bounds.bottom - gap - edge;
        const above = bounds.top - viewportTop - gap - edge;
        const opensAbove = below < 300 && above > below;

        panel.dataset.placement = opensAbove ? 'above' : 'below';
        panel.style.left = left + 'px';
        panel.style.top = (opensAbove ? bounds.top - gap : bounds.bottom + gap) + 'px';
        panel.style.setProperty('--picker-width', width + 'px');
        panel.style.setProperty('--picker-space', (opensAbove ? above : below) + 'px');
        panel.style.setProperty('--picker-arrow-x', Math.max(16, Math.min(anchorX - left, width - 16)) + 'px');
    }

    function closePicker() {
        observer.disconnect();
        if (session && panel.isConnected) panel.hidePopover?.();
        session?.abort();
        session = undefined;
        loading = false;
        nextPage = '';
        panel.hidden = true;
        picker.setAttribute('aria-expanded', 'false');
        list.removeAttribute('aria-busy');
        list.replaceChildren();
        panel.querySelector('[data-image-library-error]').textContent = '';
        if (openLibrary?.panel === panel) openLibrary = undefined;
    }

    picker.addEventListener('click', () => {
        if (session) {
            closePicker();
            return;
        }
        openLibrary?.close();
        openLibrary = { panel, picker, close: closePicker };
        session = new AbortController();
        positionPicker();
        if (!session) return;
        panel.hidden = false;
        panel.showPopover?.();
        picker.setAttribute('aria-expanded', 'true');
        scroll.scrollTop = 0;
        const { signal } = session;
        window.addEventListener('scroll', event => {
            if (!(event.target instanceof Node) || !panel.contains(event.target)) positionPicker();
        }, { capture: true, passive: true, signal });
        window.addEventListener('resize', positionPicker, { signal });
        window.visualViewport?.addEventListener('resize', positionPicker, { signal });
        window.visualViewport?.addEventListener('scroll', positionPicker, { signal });
        loadImages();
    });
    retry.addEventListener('click', () => {
        if (picker.disabled) return;
        scroll.focus();
        loadImages();
    });
    upload.addEventListener('click', () => {
        closePicker();
        picker.focus();
        input.click();
    });
    field.addEventListener('keydown', event => {
        if (event.key !== 'Escape' || !session) return;
        event.stopPropagation();
        closePicker();
        picker.focus();
    });
    field.addEventListener('focusout', event => {
        if (session && event.relatedTarget && !field.contains(event.relatedTarget)) closePicker();
    });
    return closePicker;
}

function bindImageUpload(input) {
    const field = input.closest('[data-image-upload-field]');
    const picker = field.querySelector('[data-image-picker]');
    const count = field.querySelector('[data-forum-upload-count]');
    const names = field.querySelector('[data-forum-upload-names]');
    const actions = field.querySelector('[data-forum-upload-actions]');
    const status = field.querySelector('[data-forum-upload-error]');
    const libraryCount = field.querySelector('[data-image-library-count]');
    const libraryError = field.querySelector('[data-image-library-error]');
    const list = field.querySelector('[data-image-library-list]');
    const saved = new Map();
    const references = field.querySelector('[data-image-references]') || document.createElement('div');
    const canCombine = typeof DataTransfer === 'function';
    let files = Array.from(input.files);

    references.dataset.imageReferences = '';
    references.hidden = true;
    field.appendChild(references);
    input.hidden = true;
    picker.hidden = false;

    function renderSelection() {
        const total = saved.size + files.length;
        count.textContent = total + ' / 3';
        count.hidden = total === 0;
        libraryCount.textContent = total + ' of 3 attached';
        names.textContent = [...saved.values(), ...files.map(file => file.name)].join(', ');
        actions.hidden = total === 0;
        list.querySelectorAll('[aria-pressed]').forEach(card => {
            card.setAttribute('aria-pressed', saved.has(card.dataset.savedImage));
        });
        references.replaceChildren(...Array.from(saved.keys(), id => {
            const reference = document.createElement('input');
            reference.type = 'hidden';
            reference.name = 'saved_images';
            reference.value = id;
            return reference;
        }));
    }

    function toggleImage(id, name) {
        if (saved.has(id)) {
            saved.delete(id);
        } else if (files.length + saved.size >= 3) {
            libraryError.textContent = 'Attach up to 3 images.';
            return;
        } else {
            saved.set(id, name);
        }
        status.textContent = '';
        libraryError.textContent = '';
        renderSelection();
    }
    const closePicker = bindImageLibrary(field, picker, input, saved, toggleImage);

    function clearSelection() {
        input.value = '';
        files = [];
        saved.clear();
        status.textContent = '';
        libraryError.textContent = '';
        renderSelection();
    }

    input.addEventListener('change', () => {
        const chosen = Array.from(input.files);
        const next = canCombine ? files.slice() : [];
        chosen.forEach(file => {
            const duplicate = next.some(existing => existing.name === file.name && existing.size === file.size &&
                existing.lastModified === file.lastModified);
            if (!duplicate) next.push(file);
        });

        let error = '';
        if (next.length + saved.size > 3) {
            error = 'Attach up to 3 images.';
        } else if (next.some(file => file.size > 2 * 1024 * 1024)) {
            error = 'Each image must be 2 MB or smaller.';
        }

        if (canCombine) {
            if (!error) files = next;
            const transfer = new DataTransfer();
            files.forEach(file => transfer.items.add(file));
            input.files = transfer.files;
        } else {
            if (error) input.value = '';
            files = error ? [] : chosen;
        }

        status.textContent = error;
        renderSelection();
    });
    field.querySelector('[data-forum-upload-clear]').addEventListener('click', () => {
        clearSelection();
        picker.focus();
    });
    input.form.addEventListener('reset', () => {
        clearSelection();
        closePicker();
    });
    closePicker();
    renderSelection();
}

export function initImageUploads(root) {
    root.querySelectorAll('[data-forum-upload]').forEach(input => {
        if (input.imageUploadBound) return;
        input.imageUploadBound = true;
        bindImageUpload(input);
    });
}

document.addEventListener('click', event => {
    if (openLibrary && !openLibrary.panel.contains(event.target) && !openLibrary.picker.contains(event.target)) {
        openLibrary.close();
    }
});
document.addEventListener('htmx:beforeHistorySave', () => openLibrary?.close());
document.addEventListener('htmx:beforeCleanupElement', event => {
    if (openLibrary && event.target.contains(openLibrary.panel)) openLibrary.close();
});

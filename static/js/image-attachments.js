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

function bindImageUpload(input) {
    var field = input.closest('[data-image-upload-field], .crtfld');
    var picker = field.querySelector('[data-image-picker]');
    var count = field.querySelector('[data-forum-upload-count]');
    var names = field.querySelector('[data-forum-upload-names]');
    var actions = field.querySelector('[data-forum-upload-actions]');
    var clear = field.querySelector('[data-forum-upload-clear]');
    var status = field.querySelector('[data-forum-upload-error]');
    var files = Array.from(input.files);
    var canCombine = typeof DataTransfer === 'function';

    var renderSelection = function () {
        count.textContent = files.length + ' / 3';
        count.hidden = false;
        names.textContent = files.map(function (file) { return file.name; }).join(', ');
        actions.hidden = files.length === 0;
    };
    var clearSelection = function () {
        input.value = '';
        input.setCustomValidity('');
        files = [];
        status.textContent = '';
        renderSelection();
    };

    input.addEventListener('change', function () {
        var chosen = Array.from(input.files);
        var next = canCombine ? files.slice() : [];
        chosen.forEach(function (file) {
            var duplicate = next.some(function (existing) {
                return existing.name === file.name && existing.size === file.size &&
                    existing.lastModified === file.lastModified;
            });
            if (!duplicate) next.push(file);
        });

        var error = '';
        if (next.length > 3) {
            error = 'Attach up to 3 images.';
        } else if (next.some(function (file) { return file.size > 2 * 1024 * 1024; })) {
            error = 'Each image must be 2 MB or smaller.';
        }

        if (canCombine) {
            if (!error) files = next;
            var transfer = new DataTransfer();
            files.forEach(function (file) { transfer.items.add(file); });
            input.files = transfer.files;
        } else {
            files = chosen;
            input.setCustomValidity(error);
        }

        status.textContent = error;
        renderSelection();
    });
    clear.addEventListener('click', function () {
        clearSelection();
        (picker || input).focus();
    });
    if (picker) picker.addEventListener('click', function () { input.click(); });
    input.form.addEventListener('reset', clearSelection);
    renderSelection();
}

export function initImageUploads(root) {
    root.querySelectorAll('[data-forum-upload]').forEach(function (input) {
        if (input.imageUploadBound) return;
        input.imageUploadBound = true;
        bindImageUpload(input);
    });
}

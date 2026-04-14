// wasitdown.dev — minimal JS. No framework.

(function () {
  'use strict';

  // --- Dark mode toggle ---
  var toggleBtn = document.getElementById('dark-toggle');
  if (toggleBtn) {
    var html = document.documentElement;
    function updateIcon() {
      toggleBtn.textContent = html.classList.contains('dark') ? '☀️' : '🌙';
    }
    updateIcon();
    toggleBtn.addEventListener('click', function () {
      html.classList.toggle('dark');
      localStorage.setItem('theme', html.classList.contains('dark') ? 'dark' : 'light');
      updateIcon();
    });
  }

  // --- Date page navigation ---
  const dateInput = document.getElementById('date-picker');
  if (dateInput) {
    dateInput.addEventListener('change', function () {
      const val = this.value; // yyyy-mm-dd
      if (val) {
        window.location.href = '/date/' + val;
      }
    });
  }

  // --- Provider filter on index page ---
  const filterInput = document.getElementById('provider-filter');
  if (filterInput) {
    filterInput.addEventListener('input', function () {
      const query = this.value.toLowerCase().trim();
      document.querySelectorAll('[data-provider]').forEach(function (el) {
        const name = (el.getAttribute('data-provider') || '').toLowerCase();
        el.style.display = !query || name.includes(query) ? '' : 'none';
      });
    });
  }

  // --- Impact filter on provider/date pages ---
  document.querySelectorAll('[data-impact-filter]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      const filter = this.getAttribute('data-impact-filter');
      // Toggle active state
      document.querySelectorAll('[data-impact-filter]').forEach(function (b) {
        b.classList.remove('ring-2', 'ring-brand');
      });
      this.classList.add('ring-2', 'ring-brand');

      document.querySelectorAll('[data-impact]').forEach(function (row) {
        const impact = row.getAttribute('data-impact');
        row.style.display = filter === 'all' || impact === filter ? '' : 'none';
      });
    });
  });

  // --- Lazy load AdSense on scroll ---
  if ('IntersectionObserver' in window) {
    const adObserver = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          const ad = entry.target;
          if (!ad.getAttribute('data-ad-loaded')) {
            ad.setAttribute('data-ad-loaded', '1');
            (window.adsbygoogle = window.adsbygoogle || []).push({});
          }
          adObserver.unobserve(ad);
        }
      });
    }, { rootMargin: '200px' });

    document.querySelectorAll('.adsbygoogle').forEach(function (ad) {
      adObserver.observe(ad);
    });
  }
})();

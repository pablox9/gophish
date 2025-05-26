document.addEventListener('DOMContentLoaded', function () {
    const darkModeToggle = document.getElementById('dark_mode_toggle');
    const body = document.body;
    
    // Ensure darkModeToggle exists before adding listener (it might not be on login page, etc.)
    if (darkModeToggle) {
        const darkModeIcon = darkModeToggle.querySelector('i.fa');
        const darkModeText = darkModeToggle.querySelector('span.dark-mode-text'); // Get the text span

        const applyDarkModePreference = () => {
            const isDarkMode = localStorage.getItem('darkMode') === 'enabled';
            const stylesheetLink = document.getElementById('dark-mode-stylesheet');

            if (isDarkMode) {
                body.classList.add('dark-mode');
                if (darkModeIcon) {
                    darkModeIcon.classList.remove('fa-moon-o');
                    darkModeIcon.classList.add('fa-sun-o');
                }
                if (darkModeText) {
                    darkModeText.textContent = 'Disable Dark Mode';
                }
                if (!stylesheetLink) {
                    const link = document.createElement('link');
                    link.rel = 'stylesheet';
                    link.id = 'dark-mode-stylesheet';
                    link.href = '/static/css/dist/dark_mode.css'; 
                    document.head.appendChild(link);
                }
            } else {
                body.classList.remove('dark-mode');
                if (darkModeIcon) {
                    darkModeIcon.classList.remove('fa-sun-o');
                    darkModeIcon.classList.add('fa-moon-o');
                }
                if (darkModeText) {
                    darkModeText.textContent = 'Enable Dark Mode';
                }
                if (stylesheetLink) {
                    stylesheetLink.remove();
                }
            }
        };

        darkModeToggle.addEventListener('click', function (e) {
            e.preventDefault(); // Prevent default anchor action
            body.classList.toggle('dark-mode');
            if (body.classList.contains('dark-mode')) {
                localStorage.setItem('darkMode', 'enabled');
            } else {
                localStorage.setItem('darkMode', 'disabled');
            }
            applyDarkModePreference();
        });

        // Apply preference on initial load
        applyDarkModePreference();
    }
});

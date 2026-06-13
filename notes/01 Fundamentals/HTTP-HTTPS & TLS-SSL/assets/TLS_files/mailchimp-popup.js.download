// Mailchimp popup initialization
function initMailchimpPopup() {
    // Check if user has already subscribed
    if (localStorage.getItem('subscribedToNewsletter')) {
        return; // Don't show popup if already subscribed
    }
    
    setTimeout(function() {
        var popupElement = document.getElementById('mc_embed_shell');
        var isPopupVisible = false;

        if (!popupElement) {
            return;
        }

        function showPopup() {
            if (!isPopupVisible && !localStorage.getItem('subscribedToNewsletter')) {
                popupElement.style.display = 'flex';
                isPopupVisible = true;
            }
        }

        function hidePopup() {
            popupElement.style.display = 'none';
            isPopupVisible = false;
        }

        function handleSubscribeSuccess() {
            localStorage.setItem('subscribedToNewsletter', 'true');
            hidePopup();
            
            // Show success message
            var successMessage = document.createElement('div');
            successMessage.className = 'subscription-success-message';
            successMessage.textContent = '✓ Successfully subscribed!';
            document.body.appendChild(successMessage);
            
            // Remove success message after 2 seconds
            setTimeout(function() {
                successMessage.style.opacity = '0';
                setTimeout(function() {
                    successMessage.remove();
                }, 300);
            }, 2000);
        }

        // Handle form submission
        var form = document.getElementById('mc-embedded-subscribe-form');
        if (form) {
            // Handle Mailchimp response
            var successResponse = document.getElementById('mce-success-response');
            var observer = new MutationObserver(function(mutations) {
                mutations.forEach(function(mutation) {
                    if (mutation.target.style.display === 'block') {
                        handleSubscribeSuccess();
                    }
                });
            });

            observer.observe(successResponse, { attributes: true, attributeFilter: ['style'] });

            // Also handle the form submission
            form.addEventListener('submit', function(e) {
                var email = document.getElementById('mce-EMAIL').value;
                if (email && email.includes('@')) {
                    // Show immediate feedback
                    handleSubscribeSuccess();
                }
            });
        }

        // Handle close button click
        var closeButton = document.querySelector('.close-popup');
        if (closeButton) {
            closeButton.onclick = function(e) {
                e.preventDefault();
                hidePopup();
            };
        }

        // Handle click outside
        popupElement.onclick = function(e) {
            if (e.target === popupElement) {
                hidePopup();
            }
        };

        // Show first popup after a short delay
        setTimeout(showPopup, 2000);
    }, 1000);
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initMailchimpPopup);
} else {
    initMailchimpPopup();
}

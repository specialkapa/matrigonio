class StarfieldComponent extends HTMLElement {
    constructor() {
        super();

        // Create shadow DOM
        this.attachShadow({ mode: 'open' });

        // Default values
        this._speed = 32;
        this._starCount = 2000;
        this.stars = [];
        this.animationId = null;

        // Star colors (bright white)
        this.starColors = [
            '#FFFFFF',
            '#FFFFFF',
            '#FFFFFF',
            '#FAFAFA',
            '#F5F5F5',
            '#FFFFFE'
        ];
    }

    // Observed attributes
    static get observedAttributes() {
        return ['speed', 'star-count'];
    }

    // Called when attributes change
    attributeChangedCallback(name, oldValue, newValue) {
        if (oldValue === newValue) return;

        switch (name) {
            case 'speed': {
                const n = parseFloat(newValue);
                this._speed = Number.isNaN(n) ? 32 : n;
                break;
            }
            case 'star-count': {
                const n = parseInt(newValue);
                this._starCount = Number.isNaN(n) ? 2000 : n;
                if (this.isConnected) {
                    this.syncStarCount();
                }
                break;
            }
        }
    }

    // Called when element is added to DOM
    connectedCallback() {
        this.setupComponent();
        this.init();
        this.animate();
    }

    // Called when element is removed from DOM
    disconnectedCallback() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
        }
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
        }
    }

    setupComponent() {
        // Create the shadow DOM structure
        this.shadowRoot.innerHTML = `
            <style>
                :host {
                    display: block;
                    position: relative;
                    width: 100%;
                    height: 100%;
                    background: #000;
                }

                canvas {
                    display: block;
                    width: 100%;
                    height: 100%;
                }
            </style>
            <canvas></canvas>
        `;

        this.canvas = this.shadowRoot.querySelector('canvas');
        this.ctx = this.canvas.getContext('2d');
    }

    init() {
        const s = parseFloat(this.getAttribute('speed'));
        this._speed = Number.isNaN(s) ? 32 : s;
        const c = parseInt(this.getAttribute('star-count'));
        this._starCount = Number.isNaN(c) ? 2000 : c;

        this.resize();
        this.createStars();

        // Set up resize observer
        this.resizeObserver = new ResizeObserver(() => this.resize());
        this.resizeObserver.observe(this);
    }

    resize() {
        const rect = this.getBoundingClientRect();
        this.width = this.canvas.width = rect.width;
        this.height = this.canvas.height = rect.height;
        this.centerX = this.width / 2;
        this.centerY = this.height / 2;
        this.maxDepth = 1000;
    }

    createStars() {
        this.stars = [];
        for (let i = 0; i < this._starCount; i++) {
            this.stars.push(this.createStar());
        }
    }

    createStar() {
        return {
            x: (Math.random() - 0.5) * 2000,
            y: (Math.random() - 0.5) * 2000,
            z: Math.random() * this.maxDepth,
            color: this.starColors[Math.floor(Math.random() * this.starColors.length)],
            size: Math.random() * 1.5
        };
    }

    // Incrementally grow or trim the star array to match _starCount.
    // Avoids the full-rebuild flicker of createStars() when the count
    // is animated via signals.
    syncStarCount() {
        const diff = this._starCount - this.stars.length;
        if (diff > 0) {
            for (let i = 0; i < diff; i++) this.stars.push(this.createStar());
        } else if (diff < 0) {
            this.stars.length = this._starCount;
        }
    }

    updateStar(star) {
        // Move star closer
        star.z -= this._speed;

        // Reset star if it goes behind camera
        if (star.z <= 0) {
            star.x = (Math.random() - 0.5) * 2000;
            star.y = (Math.random() - 0.5) * 2000;
            star.z = this.maxDepth;
        }
    }

    drawStar(star) {
        // Calculate screen position
        const x = star.x / star.z * this.centerX + this.centerX;
        const y = star.y / star.z * this.centerY + this.centerY;

        // Check if star is within canvas bounds
        if (x < 0 || x > this.width || y < 0 || y > this.height) {
            return;
        }

        // Calculate size based on depth
        const size = (1 - star.z / this.maxDepth) * 2 * star.size;

        // Calculate opacity based on depth
        const opacity = 1 - star.z / this.maxDepth;

        // Draw star as circle
        this.ctx.fillStyle = star.color;
        this.ctx.globalAlpha = opacity;
        this.ctx.beginPath();
        this.ctx.arc(x, y, size, 0, Math.PI * 2);
        this.ctx.fill();
        this.ctx.globalAlpha = 1;
    }

    clear() {
        this.ctx.fillStyle = '#000';
        this.ctx.fillRect(0, 0, this.width, this.height);
    }

    animate() {
        this.clear();

        // Update and draw all stars
        for (let star of this.stars) {
            this.updateStar(star);
            this.drawStar(star);
        }

        this.animationId = requestAnimationFrame(() => this.animate());
    }
}

// Register the custom element
customElements.define('star-field', StarfieldComponent);


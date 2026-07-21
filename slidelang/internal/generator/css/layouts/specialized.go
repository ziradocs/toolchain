// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package layouts

// SpecializedCSS provides CSS for specialized presentation layouts
const SpecializedCSS = `/* === SPECIALIZED LAYOUTS === */

/* Hero Layout */
.slide.hero-layout {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--gradient-bg);
    color: var(--text-on-primary);
    padding: 4rem;
}

.slide.hero-layout .hero-content {
    flex: 1;
    max-width: 50%;
}

.slide.hero-layout .hero-content h1 {
    font-size: 4rem;
    font-weight: 700;
    margin-bottom: 2rem;
    line-height: 1.1;
    text-shadow: 0 2px 4px rgba(0,0,0,0.3);
}

.slide.hero-layout .hero-content .hero-subtitle {
    font-size: 1.5rem;
    font-weight: 300;
    margin-bottom: 2rem;
    opacity: 0.9;
}

.slide.hero-layout .hero-content .hero-cta {
    display: inline-block;
    background: var(--accent-color);
    color: var(--text-on-accent);
    padding: 1rem 2rem;
    border-radius: var(--border-radius);
    text-decoration: none;
    font-weight: 600;
    font-size: 1.1rem;
    transition: var(--transition);
    box-shadow: var(--shadow-main);
}

.slide.hero-layout .hero-content .hero-cta:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-lg);
}

.slide.hero-layout .hero-visual {
    flex: 1;
    max-width: 45%;
    text-align: center;
}

.slide.hero-layout .hero-visual img {
    max-width: 100%;
    height: auto;
    border-radius: var(--border-radius-lg);
    box-shadow: var(--shadow-lg);
}

/* Comparison Layout */
.slide.comparison-layout {
    padding: 3rem;
}

.slide.comparison-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.comparison-layout .comparison-container {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 3rem;
    height: 70%;
}

.slide.comparison-layout .comparison-side {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 2rem;
    box-shadow: var(--shadow-main);
    position: relative;
    transition: var(--transition);
}

.slide.comparison-layout .comparison-side:hover {
    transform: translateY(-5px);
    box-shadow: var(--shadow-lg);
}

.slide.comparison-layout .comparison-side.vs-left {
    border-top: 4px solid var(--danger-color);
}

.slide.comparison-layout .comparison-side.vs-right {
    border-top: 4px solid var(--success-color);
}

.slide.comparison-layout .comparison-side h2 {
    text-align: center;
    margin-bottom: 1.5rem;
    font-size: 1.8rem;
}

.slide.comparison-layout .comparison-side.vs-left h2 {
    color: var(--danger-color);
}

.slide.comparison-layout .comparison-side.vs-right h2 {
    color: var(--success-color);
}

.slide.comparison-layout .comparison-list {
    list-style: none;
    padding: 0;
}

.slide.comparison-layout .comparison-list li {
    padding: 0.8rem 0;
    border-bottom: 1px solid var(--bg-light);
    position: relative;
    padding-left: 2rem;
}

.slide.comparison-layout .comparison-list li:before {
    position: absolute;
    left: 0;
    top: 0.8rem;
    font-weight: bold;
}

.slide.comparison-layout .vs-left .comparison-list li:before {
    content: "✗";
    color: var(--danger-color);
}

.slide.comparison-layout .vs-right .comparison-list li:before {
    content: "✓";
    color: var(--success-color);
}

/* Timeline Layout */
.slide.timeline-layout {
    padding: 3rem;
}

.slide.timeline-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.timeline-layout .timeline {
    position: relative;
    max-width: 800px;
    margin: 0 auto;
}

.slide.timeline-layout .timeline::before {
    content: '';
    position: absolute;
    left: 50%;
    top: 0;
    bottom: 0;
    width: 4px;
    background: var(--primary-color);
    transform: translateX(-50%);
}

.slide.timeline-layout .timeline-item {
    position: relative;
    margin-bottom: 3rem;
    width: 45%;
}

.slide.timeline-layout .timeline-item:nth-child(odd) {
    left: 0;
}

.slide.timeline-layout .timeline-item:nth-child(even) {
    left: 55%;
}

.slide.timeline-layout .timeline-item::before {
    content: '';
    position: absolute;
    width: 20px;
    height: 20px;
    background: var(--primary-color);
    border: 4px solid white;
    border-radius: 50%;
    box-shadow: 0 0 0 4px var(--primary-color);
    top: 1rem;
    z-index: 1;
}

.slide.timeline-layout .timeline-item:nth-child(odd)::before {
    right: -35px;
}

.slide.timeline-layout .timeline-item:nth-child(even)::before {
    left: -35px;
}

.slide.timeline-layout .timeline-content {
    background: var(--bg-white);
    padding: 1.5rem;
    border-radius: var(--border-radius);
    box-shadow: var(--shadow-main);
    transition: var(--transition);
}

.slide.timeline-layout .timeline-content:hover {
    transform: translateY(-3px);
    box-shadow: var(--shadow-lg);
}

.slide.timeline-layout .timeline-date {
    font-size: 0.9rem;
    color: var(--primary-color);
    font-weight: 600;
    margin-bottom: 0.5rem;
}

.slide.timeline-layout .timeline-title {
    font-size: 1.3rem;
    color: var(--secondary-color);
    margin-bottom: 0.5rem;
    font-weight: 600;
}

.slide.timeline-layout .timeline-description {
    color: var(--text-color);
    line-height: 1.5;
}

/* Testimonial Layout */
.slide.testimonial-layout {
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    background: linear-gradient(135deg, var(--bg-light), white);
    padding: 4rem;
}

.slide.testimonial-layout .testimonial-container {
    max-width: 800px;
}

.slide.testimonial-layout .testimonial-quote {
    font-size: 2rem;
    font-style: italic;
    color: var(--secondary-color);
    margin-bottom: 2rem;
    line-height: 1.4;
    position: relative;
}

.slide.testimonial-layout .testimonial-quote::before {
    content: '"';
    font-size: 4rem;
    color: var(--primary-color);
    position: absolute;
    top: -1rem;
    left: -2rem;
    font-family: serif;
}

.slide.testimonial-layout .testimonial-quote::after {
    content: '"';
    font-size: 4rem;
    color: var(--primary-color);
    position: absolute;
    bottom: -2rem;
    right: -2rem;
    font-family: serif;
}

.slide.testimonial-layout .testimonial-author {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 1.5rem;
}

.slide.testimonial-layout .testimonial-avatar {
    width: 80px;
    height: 80px;
    border-radius: 50%;
    object-fit: cover;
    border: 4px solid var(--primary-color);
}

.slide.testimonial-layout .testimonial-info h3 {
    font-size: 1.3rem;
    color: var(--secondary-color);
    margin-bottom: 0.3rem;
}

.slide.testimonial-layout .testimonial-info p {
    color: var(--text-light);
    font-size: 1rem;
}

/* Portfolio Layout */
.slide.portfolio-layout {
    padding: 3rem;
}

.slide.portfolio-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.portfolio-layout .portfolio-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: 2rem;
}

.slide.portfolio-layout .portfolio-item {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    overflow: hidden;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
}

.slide.portfolio-layout .portfolio-item:hover {
    transform: translateY(-5px);
    box-shadow: var(--shadow-lg);
}

.slide.portfolio-layout .portfolio-item img {
    width: 100%;
    height: 200px;
    object-fit: cover;
}

.slide.portfolio-layout .portfolio-content {
    padding: 1.5rem;
}

.slide.portfolio-layout .portfolio-title {
    font-size: 1.3rem;
    color: var(--secondary-color);
    margin-bottom: 0.5rem;
    font-weight: 600;
}

.slide.portfolio-layout .portfolio-description {
    color: var(--text-color);
    line-height: 1.5;
    margin-bottom: 1rem;
}

.slide.portfolio-layout .portfolio-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
}

.slide.portfolio-layout .portfolio-tag {
    background: var(--primary-color);
    color: var(--text-on-primary);
    padding: 0.3rem 0.8rem;
    border-radius: 15px;
    font-size: 0.8rem;
    font-weight: 500;
}

/* Team Layout */
.slide.team-layout {
    padding: 3rem;
}

.slide.team-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.team-layout .team-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 2rem;
    max-width: 1000px;
    margin: 0 auto;
}

.slide.team-layout .team-member {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 2rem;
    text-align: center;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
}

.slide.team-layout .team-member:hover {
    transform: translateY(-5px);
    box-shadow: var(--shadow-lg);
}

.slide.team-layout .team-avatar {
    width: 120px;
    height: 120px;
    border-radius: 50%;
    object-fit: cover;
    margin: 0 auto 1rem;
    border: 4px solid var(--primary-color);
}

.slide.team-layout .team-name {
    font-size: 1.3rem;
    color: var(--secondary-color);
    margin-bottom: 0.5rem;
    font-weight: 600;
}

.slide.team-layout .team-role {
    color: var(--primary-color);
    font-weight: 500;
    margin-bottom: 1rem;
}

.slide.team-layout .team-bio {
    color: var(--text-color);
    line-height: 1.5;
    font-size: 0.95rem;
}

/* Pricing Layout */
.slide.pricing-layout {
    padding: 3rem;
}

.slide.pricing-layout h1 {
    text-align: center;
    margin-bottom: 3rem;
    font-size: 2.5rem;
    color: var(--secondary-color);
}

.slide.pricing-layout .pricing-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 2rem;
    max-width: 1000px;
    margin: 0 auto;
}

.slide.pricing-layout .pricing-card {
    background: var(--bg-white);
    border-radius: var(--border-radius-lg);
    padding: 2rem;
    text-align: center;
    box-shadow: var(--shadow-main);
    transition: var(--transition);
    position: relative;
}

.slide.pricing-layout .pricing-card:hover {
    transform: translateY(-5px);
    box-shadow: var(--shadow-lg);
}

.slide.pricing-layout .pricing-card.featured {
    border: 3px solid var(--primary-color);
    transform: scale(1.05);
}

.slide.pricing-layout .pricing-card.featured::before {
    content: 'POPULAR';
    position: absolute;
    top: -10px;
    left: 50%;
    transform: translateX(-50%);
    background: var(--primary-color);
    color: var(--text-on-primary);
    padding: 0.5rem 1rem;
    border-radius: var(--border-radius);
    font-size: 0.8rem;
    font-weight: 600;
}

.slide.pricing-layout .pricing-plan {
    font-size: 1.5rem;
    color: var(--secondary-color);
    margin-bottom: 1rem;
    font-weight: 600;
}

.slide.pricing-layout .pricing-price {
    font-size: 3rem;
    color: var(--primary-color);
    font-weight: 700;
    margin-bottom: 0.5rem;
}

.slide.pricing-layout .pricing-period {
    color: var(--text-light);
    margin-bottom: 2rem;
}

.slide.pricing-layout .pricing-features {
    list-style: none;
    padding: 0;
    margin-bottom: 2rem;
}

.slide.pricing-layout .pricing-features li {
    padding: 0.5rem 0;
    color: var(--text-color);
}

.slide.pricing-layout .pricing-features li:before {
    content: '✓';
    color: var(--success-color);
    font-weight: bold;
    margin-right: 0.5rem;
}

.slide.pricing-layout .pricing-cta {
    background: var(--primary-color);
    color: var(--text-on-primary);
    padding: 1rem 2rem;
    border: none;
    border-radius: var(--border-radius);
    font-weight: 600;
    cursor: pointer;
    transition: var(--transition);
    width: 100%;
}

.slide.pricing-layout .pricing-cta:hover {
    background: var(--secondary-color);
}

/* Feature Layout */
.slide.feature-layout {
    padding: 3rem;
    display: flex;
    align-items: center;
    gap: 4rem;
}

.slide.feature-layout .feature-content {
    flex: 1;
}

.slide.feature-layout .feature-content h1 {
    font-size: 2.5rem;
    color: var(--secondary-color);
    margin-bottom: 1.5rem;
}

.slide.feature-layout .feature-content .feature-description {
    font-size: 1.3rem;
    color: var(--text-color);
    line-height: 1.6;
    margin-bottom: 2rem;
}

.slide.feature-layout .feature-benefits {
    list-style: none;
    padding: 0;
}

.slide.feature-layout .feature-benefits li {
    padding: 0.8rem 0;
    padding-left: 2rem;
    position: relative;
    font-size: 1.1rem;
    color: var(--text-color);
}

.slide.feature-layout .feature-benefits li:before {
    content: '✓';
    position: absolute;
    left: 0;
    color: var(--success-color);
    font-weight: bold;
    font-size: 1.2rem;
}

.slide.feature-layout .feature-visual {
    flex: 1;
    text-align: center;
}

.slide.feature-layout .feature-visual img {
    max-width: 100%;
    height: auto;
    border-radius: var(--border-radius-lg);
    box-shadow: var(--shadow-lg);
}

/* Responsive adjustments for specialized layouts */
@media (max-width: 768px) {
    .slide.hero-layout {
        flex-direction: column;
        text-align: center;
        padding: 2rem;
    }
    
    .slide.hero-layout .hero-content,
    .slide.hero-layout .hero-visual {
        max-width: 100%;
    }
    
    .slide.hero-layout .hero-content h1 {
        font-size: 2.5rem;
    }
    
    .slide.comparison-layout .comparison-container {
        grid-template-columns: 1fr;
        gap: 2rem;
    }
    
    .slide.timeline-layout .timeline::before {
        left: 20px;
    }
    
    .slide.timeline-layout .timeline-item {
        width: calc(100% - 60px);
        left: 60px !important;
    }
    
    .slide.timeline-layout .timeline-item::before {
        left: -35px !important;
    }
    
    .slide.feature-layout {
        flex-direction: column;
        gap: 2rem;
    }
    
    .slide.pricing-layout .pricing-grid,
    .slide.team-layout .team-grid,
    .slide.portfolio-layout .portfolio-grid {
        grid-template-columns: 1fr;
    }
}
`

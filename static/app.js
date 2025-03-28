class LetterboxdStats {
    constructor() {
        this.endpoints = {
            watched: '/api/data?type=watched',
            watchlist: '/api/data?type=watchlist',
            reviews: '/api/data?type=reviews',
            ratings: '/api/data?type=ratings',
            comments: '/api/data?type=comments'
        };
        
        this.currentView = 'stats';
        this.dataExplorer = new DataExplorer();
        this.initEventListeners();
        this.init();
        this.initTheme();
    }

    initEventListeners() {
        document.querySelectorAll('.menu-item').forEach(item => {
            item.addEventListener('click', async (e) => {
                const viewType = e.target.dataset.view;
                const file = e.target.dataset.file;
    
                if (viewType) {
                    this.switchView(viewType);
                } else if (file) {
                    this.switchView('table');
                    await this.dataExplorer.loadAndDisplayData(file);
                }
            });
        });

    }

    switchView(viewType) {
        this.currentView = viewType;
        
        document.querySelectorAll('.menu-item').forEach(item => {
            item.classList.toggle('active', item.dataset.view === viewType);
        });
    
        const statsView = document.querySelector('.stats-view');
        const tableContainer = document.getElementById('table-container');
        const controls = document.querySelector('.controls');
    
        if (statsView) {
            statsView.style.display = viewType === 'stats' ? 'block' : 'none';
            // Réinitialiser les erreurs
            this.hideError();
        }
        
        if (tableContainer) {
            tableContainer.style.display = viewType === 'table' ? 'block' : 'none';
        }
        
        if (controls) {
            controls.style.display = viewType === 'table' ? 'flex' : 'none';
        }
    }
    
    // Ajouter cette méthode
    hideError() {
        const errorContainer = document.querySelector('.error-container');
        if (errorContainer) {
            errorContainer.style.display = 'none';
            errorContainer.textContent = '';
        }
    }

    async init() {
        try {
            this.showLoading();
            const data = await this.loadAllData();
            this.processData(data);
            this.switchView('stats');
        } catch (error) {
            this.showError(error.message);
        } finally {
            this.hideLoading();
        }
    }

    async loadAllData() {
        const data = {};
        
        await Promise.all(Object.entries(this.endpoints).map(async ([key, url]) => {
            try {
                const response = await fetch(url);
                if (!response.ok) throw new Error(`HTTP error ${response.status}`);
                data[key] = await response.json();
            } catch (error) {
                console.warn(`Failed to load ${key}: ${error.message}`);
                data[key] = [];
            }
        }));

        return data;
    }

    async fetchCSV(file) {
        try {
            const response = await fetch(file);
            if (!response.ok) throw new Error(`HTTP error ${response.status}`);
            const raw = await response.text();
            return this.parseCSV(raw);
        } catch (error) {
            throw new Error(`Failed to load ${file}: ${error.message}`);
        }
    }

    parseCSV(csvText) {
        const lines = csvText.split('\n').filter(line => line.trim() !== '');
        if (lines.length < 1) throw new Error('Empty CSV file');
        
        const headers = lines[0].split(',').map(h => h.trim());
        return lines.slice(1).map(line => {
            const values = line.split(',').map(v => v.trim());
            if (values.length !== headers.length) throw new Error('CSV format mismatch');
            
            return headers.reduce((obj, header, index) => {
                obj[header] = values[index];
                return obj;
            }, {});
        });
    }

    initTheme() {
        const savedTheme = localStorage.getItem('letterboxd-theme') || 'default';
        document.documentElement.setAttribute('data-theme', savedTheme);
        
        const themeSelect = document.getElementById('theme-select');
        if (themeSelect) {
            themeSelect.value = savedTheme;
            themeSelect.addEventListener('change', (e) => {
                const theme = e.target.value;
                document.documentElement.setAttribute('data-theme', theme);
                localStorage.setItem('letterboxd-theme', theme);
            });
        }
    }

    processData(data) {
        this.processWatched(data.watched);
        this.processWatchlist(data.watchlist);
        this.processRatingsAndReviews(data.ratings, data.reviews);
        this.processComments(data.comments);
    }

    processWatched(watched) {
        if (!watched.length) return;
        
        document.getElementById('total-watched').textContent = watched.length;
        
        const years = watched.reduce((acc, entry) => {
            acc[entry.Year] = (acc[entry.Year] || 0) + 1;
            return acc;
        }, {});
        
        const [topYear, count] = Object.entries(years)
            .sort((a, b) => b[1] - a[1])[0] || [];
            
        if (topYear) {
            document.getElementById('top-year').textContent = `${topYear} (${count})`;
        }
    }

    processWatchlist(watchlist) {
        document.getElementById('watchlist-length').textContent = watchlist.length;
    }

    processRatingsAndReviews(ratings, reviews) {
        const allRatings = [
            ...ratings.map(r => parseFloat(r.Rating)),
            ...reviews.filter(r => r.Rating).map(r => parseFloat(r.Rating))
        ].filter(r => !isNaN(r));
        
        if (allRatings.length) {
            const avg = allRatings.reduce((a, b) => a + b, 0) / allRatings.length;
            document.getElementById('average-rating').textContent = avg.toFixed(1);
        }
        
        document.getElementById('total-reviews').textContent = reviews.length;
        
        const tags = reviews.flatMap(r => 
            r.Tags ? r.Tags.split(',').map(t => t.trim()) : []
        );
        const tagCounts = tags.reduce((acc, tag) => {
            acc[tag] = (acc[tag] || 0) + 1;
            return acc;
        }, {});
        
        const topTags = Object.entries(tagCounts)
            .sort((a, b) => b[1] - a[1])
            .slice(0, 3)
            .map(([tag, count]) => `${tag} (${count})`)
            .join(', ');
            
        document.getElementById('top-tags').textContent = topTags || 'None';
    }

    processComments(comments) {
        // Additional comment statistics can be added here
    }



    showLoading() {
        document.querySelector('.loading-spinner').style.display = 'block';
    }

    hideLoading() {
        document.querySelector('.loading-spinner').style.display = 'none';
    }

    showError(message) {
        const errorContainer = document.querySelector('.error-container');
        errorContainer.style.display = 'block';
        errorContainer.textContent = `Error: ${message}`;
    }
}

class DataExplorer {
    constructor() {
        this.currentData = [];
        this.currentFile = '';
        this.currentSort = { column: '', order: 'asc' };
        this.initialized = false; // Nouveau flag
    }

    async loadAndDisplayData(file) {
        try {
            this.currentFile = file;
            const response = await fetch(`/api/data?type=${file}`);
            const data = await response.json();
            this.currentData = data;
            
            // Initialiser les contrôles uniquement quand nécessaire
            if (!this.initialized) {
                this.initializeControls();
                this.initialized = true;
            }
            
            this.renderTable(data);
            this.populateFilterColumns(data[0]);
        } catch (error) {
            console.error('Error loading data:', error);
        }
    }

    initializeControls() {
        const search = document.getElementById('search');
        const filterColumn = document.getElementById('filter-column');
        
        if (search && filterColumn) {
            // Nettoyer les anciens écouteurs
            search.replaceWith(search.cloneNode(true));
            filterColumn.replaceWith(filterColumn.cloneNode(true));
            
            // Réattacher les écouteurs
            document.getElementById('search').addEventListener('input', (e) => {
                this.filterTable(e.target.value);
            });
            
            document.getElementById('filter-column').addEventListener('change', (e) => {
                this.filterTable(document.getElementById('search').value);
            });
        }
    }

    populateFilterColumns(row) {
        const select = document.getElementById('filter-column');
        if (!select) return; // Protection supplémentaire
        
        select.innerHTML = '<option value="">All Columns</option>';
        if (row) Object.keys(row).forEach(column => {
            select.appendChild(new Option(column, column));
        });
    }

    renderTable(data) {
        const container = document.getElementById('table-container');
        if (!data || data.length === 0) {
            container.innerHTML = '<div class="no-data">Aucune donnée disponible</div>';
            return;
        }
    
        // Debug logging with enhanced details
        console.log('[DEBUG] Headers:', Object.keys(data[0]));
        console.log('[DEBUG] First row:', data[0]);
        console.log('[DEBUG] Data sample:', data.slice(0, 3));
    
        const headers = Object.keys(data[0]);
        let html = `
            <table>
                <thead>
                    <tr>
                        ${headers.map(h => 
                            `<th onclick="letterboxdApp.dataExplorer.sortTable('${h}')"
                            onmouseover="this.classList.add('hovering')"
                            onmouseout="this.classList.remove('hovering')">
                            ${h}
                            <span class="sort-arrow ${this.currentSort.column === h ? 'active' : ''}">
                            ${this.currentSort.column === h ? (this.currentSort.order === 'asc' ? '↑' : '↓') : '↕'}
                        </span></th>`).join('')}
                    </tr>
                </thead>
                <tbody>
        `;

        
    
        html += data.map(row => {
            return `
                <tr>
                    ${headers.map(header => {
                        const value = row[header] || '';
                        
                        // 1. Find URI column using multiple patterns
                        const uriHeader = Object.keys(row).find(k => 
                            k.toLowerCase().match(/(uri|url|link|movie[-_]?link)/)
                        );
                        const uri = uriHeader ? row[uriHeader] : '';
    
                        // 2. Find name column using multiple patterns
                        const nameHeader = Object.keys(row).find(k => 
                            k.toLowerCase().match(/(name|title|film|movie)/)
                        );
    
                        // 3. Debug logging for cell processing
                        console.log('[CELL DEBUG] Processing header:', header);
                        console.log('[CELL DEBUG] URI detected:', uri);
                        console.log('[CELL DEBUG] Name match:', 
                            header.toLowerCase() === nameHeader?.toLowerCase());
    
                        // 4. Create link only if ALL conditions are met
                        if (header === nameHeader && uri) {
                            console.log('[LINK CREATION] Creating link for:', value);
                            return `
                                <td>
                                    <a href="${uri}" 
                                       target="_blank" 
                                       class="movie-link"
                                       title="Ouvrir dans Letterboxd">
                                        ${value}
                                        <span class="external-icon">↗</span>
                                    </a>
                                </td>
                            `;
                        }
                        return `<td>${value}</td>`;
                    }).join('')}
                </tr>
            `;
        }).join('');
    
        html += '</tbody></table>';
        container.innerHTML = html;
        
        // Post-render debug check
        console.log('[POST-RENDER] Table HTML:', container.innerHTML);
    }

    filterTable(searchTerm) {
        const filterColumn = document.getElementById('filter-column').value;
        const filtered = this.currentData.filter(row => {
            if (!searchTerm) return true;
            if (filterColumn) {
                return (row[filterColumn] || '').toLowerCase().includes(searchTerm.toLowerCase());
            }
            return Object.values(row).some(value => 
                value.toString().toLowerCase().includes(searchTerm.toLowerCase())
            );
        });
        this.renderTable(filtered);
    }

    sortTable(column) {
        console.log('Sorting by:', column); // Debug log
        console.log('First item:', this.currentData[0]); // Debug log
        
        if (this.currentSort.column === column) {
            this.currentSort.order = this.currentSort.order === 'asc' ? 'desc' : 'asc';
        } else {
            this.currentSort = { column, order: 'asc' };
        }
    
        this.currentData.sort((a, b) => {
            const valA = a[column] || '';
            const valB = b[column] || '';
            return this.currentSort.order === 'asc' 
                ? valA.localeCompare(valB) 
                : valB.localeCompare(valA);
        });
    
        this.renderTable(this.currentData);
    }
}

// Initialize the application
const letterboxdApp = new LetterboxdStats();

document.addEventListener('DOMContentLoaded', () => {
    const toggleButton = document.getElementById('toggle-sidebar');
    const sidebar = document.querySelector('.sidebar');
    const content = document.querySelector('.content');

    toggleButton.addEventListener('click', () => {
        sidebar.classList.toggle('hidden');
        content.classList.toggle('expanded');
    });
});
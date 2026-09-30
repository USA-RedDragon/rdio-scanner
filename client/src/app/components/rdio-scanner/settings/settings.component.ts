/*
 * *****************************************************************************
 * Copyright (C) 2019-2022 Chrystian Huot <chrystian.huot@saubeo.solutions>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>
 * ****************************************************************************
 */

import { Component, inject } from '@angular/core';
import { RdioScannerSettingsService } from './settings.service';

@Component({
    selector: 'rdio-scanner-settings',
    styleUrls: [
        '../common.scss',
        './settings.component.scss',
    ],
    templateUrl: './settings.component.html',
    standalone: false
})

export class RdioScannerSettingsComponent {
    private rdioScannerSettingsService = inject(RdioScannerSettingsService)

    get disableBeep(): boolean {
        return this.rdioScannerSettingsService.disableBeep;
    }

    private set disableBeep(disableBeep: boolean) {
        this.rdioScannerSettingsService.disableBeep = disableBeep;
    }

    private get disableBeepLabel(): string {
        return this.disableBeep ? 'Disable' : 'Enable';
    }

    toggleDisableBeep(): void {
        this.disableBeep = !this.disableBeep;
    }
}

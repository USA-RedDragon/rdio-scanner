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

import { CdkDragDrop, moveItemInArray } from '@angular/cdk/drag-drop';
import { Component, Input, OnChanges, QueryList, ViewChildren, inject } from '@angular/core';
import { UntypedFormArray, UntypedFormControl, UntypedFormGroup } from '@angular/forms';
import { MatExpansionPanel } from '@angular/material/expansion';
import { RdioScannerAdminService } from '../../admin.service';

@Component({
    selector: 'rdio-scanner-admin-dir-watch',
    templateUrl: './dir-watch.component.html',
    standalone: false
})
export class RdioScannerAdminDirWatchComponent implements OnChanges {
    private adminService = inject(RdioScannerAdminService)

    @Input() form: UntypedFormArray | undefined;

    get dirWatches(): UntypedFormGroup[] {
        return this.form?.controls
            .sort((a, b) => a.value.order - b.value.order) as UntypedFormGroup[];
    }

    get systems(): UntypedFormGroup[] {
        const systems = this.form?.root.get('systems') as UntypedFormArray;

        return systems.controls as UntypedFormGroup[];
    }

    get talkgroups(): UntypedFormGroup[][] {
        return this.systems.reduce((talkgroups, system) => {
            const faTalkgroups = system.get('talkgroups') as UntypedFormArray;

            talkgroups[system.value.id] = faTalkgroups.controls as UntypedFormGroup[];

            return talkgroups;
        }, [] as UntypedFormGroup[][]);
    }

    @ViewChildren(MatExpansionPanel) private panels: QueryList<MatExpansionPanel> | undefined;

    ngOnChanges(): void {
        if (this.form) {
            this.dirWatches.flatMap((control) => this.registerOnChanges(control));
        }
    }

    add(): void {
        const dirWatch = this.adminService.newDirWatchForm({
            delay: 2000,
            deleteAfter: true,
        });

        dirWatch.markAllAsTouched();

        this.registerOnChanges(dirWatch);

        this.form?.insert(0, dirWatch);

        this.form?.markAsDirty();
    }

    closeAll(): void {
        this.panels?.forEach((panel) => panel.close());
    }

    drop(event: CdkDragDrop<UntypedFormGroup[]>): void {
        if (event.previousIndex !== event.currentIndex) {
            moveItemInArray(event.container.data, event.previousIndex, event.currentIndex);

            event.container.data.forEach((dat, idx) => dat.get('order')?.setValue(idx + 1, { emitEvent: false }));

            this.form?.markAsDirty();
        }
    }

    remove(index: number): void {
        this.form?.removeAt(index);

        this.form?.markAsDirty();
    }

    private registerOnChanges(control: UntypedFormGroup): void {
        const mask = control.get('mask') as UntypedFormControl;
        const type = control.get('type') as UntypedFormControl;

        mask.valueChanges.subscribe(() => this.validateIds(control));
        type.valueChanges.subscribe(() => this.validateIds(control));
    }

    private validateIds(control: UntypedFormGroup): void {
        const systemId = control.get('systemId');
        const talkgroupId = control.get('talkgroupId');

        systemId?.updateValueAndValidity();
        systemId?.markAsTouched();

        talkgroupId?.updateValueAndValidity();
        talkgroupId?.markAsTouched();
    }
}

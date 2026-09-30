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
import { Component, EventEmitter, Input, Output, QueryList, ViewChildren, inject, OnChanges } from '@angular/core';
import { UntypedFormArray, UntypedFormControl, UntypedFormGroup } from '@angular/forms';
import { MatExpansionPanel } from '@angular/material/expansion';
import { RdioScannerAdminService, Group, Tag } from '../../../admin.service';

@Component({
    selector: 'rdio-scanner-admin-system',
    templateUrl: './system.component.html',
    standalone: false
})
export class RdioScannerAdminSystemComponent implements OnChanges {
    private adminService = inject(RdioScannerAdminService)

    @Input() form = new UntypedFormGroup({});

    @Input() groups: Group[] = [];

    @Input() tags: Tag[] = [];

    @Output() add = new EventEmitter<void>();

    @Output() remove = new EventEmitter<void>();

    get led(): UntypedFormControl {
        return this.form.get('led') as UntypedFormControl;
    }

    get talkgroups(): UntypedFormGroup[] {
        const talkgroups = this.form.get('talkgroups') as UntypedFormArray;

        return talkgroups.controls
            .sort((a, b) => (a.value.order || 0) - (b.value.order || 0)) as UntypedFormGroup[];
    }

    get units(): UntypedFormGroup[] {
        const units = this.form.get('units') as UntypedFormArray;

        return units.controls
            .sort((a, b) => (a.value.order || 0) - (b.value.order || 0)) as UntypedFormGroup[];
    }

    ngOnChanges(): void {
    }

    @ViewChildren(MatExpansionPanel) private panels: QueryList<MatExpansionPanel> | undefined;

    addTalkgroup(): void {
        const talkgroups = this.form.get('talkgroups') as UntypedFormArray;

        talkgroups.insert(0, this.adminService.newTalkgroupForm());

        this.form.markAsDirty();
    }

    addUnit(): void {
        const units = this.form.get('units') as UntypedFormArray;

        units.insert(0, this.adminService.newUnitForm());

        this.form.markAsDirty();
    }

    blacklistTalkgroup(index: number): void {
        const talkgroup = this.talkgroups[index];

        const id = talkgroup.value.id;

        if (typeof id !== 'number') {
            return;
        }

        const blacklists = this.form?.get('blacklists') as UntypedFormControl;

        blacklists.setValue(blacklists.value?.trim() ? `${blacklists.value},${id}` : `${id}`);

        this.removeTalkgroup(index);
    }

    closeAll(): void {
        this.panels?.forEach((panel) => panel.close());
    }

    drop(event: CdkDragDrop<UntypedFormGroup[]>): void {
        if (event.previousIndex !== event.currentIndex) {
            moveItemInArray(event.container.data, event.previousIndex, event.currentIndex);

            event.container.data.forEach((dat, idx) => dat.get('order')?.setValue(idx + 1, { emitEvent: false }));

            this.form.markAsDirty();
        }
    }

    removeTalkgroup(index: number): void {
        const talkgroups = this.form.get('talkgroups') as UntypedFormArray;

        talkgroups.removeAt(index);

        talkgroups.markAsDirty();
    }

    removeUnit(index: number): void {
        const units = this.form.get('units') as UntypedFormArray;

        units.removeAt(index);

        units.markAsDirty();
    }
}

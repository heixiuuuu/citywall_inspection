"""合并 MSD 与砖石裂缝标注，保留目标场景划分并隔离不明标签。"""
import argparse
from collections import Counter
import hashlib
import io
import json
import math
import os
from pathlib import Path
import shutil
import zipfile

from PIL import Image
import yaml


def file_hash(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def pixel_hash(image):
    rgb = image.convert('RGB')
    return hashlib.sha256(rgb.tobytes() + f'{rgb.width}:{rgb.height}'.encode()).hexdigest()


def parse_labels(text, names, normalizations=None):
    result = []
    for line in text.splitlines():
        if not line.strip():
            continue
        fields = line.split()
        if len(fields) != 5 or int(fields[0]) not in names:
            raise ValueError('invalid label fields or class index')
        x, y, w, h = map(float, fields[1:])
        if not all(math.isfinite(value) for value in (x, y, w, h)) or not (0 <= x <= 1 and 0 <= y <= 1 and 0 < w <= 1 and 0 < h <= 1):
            raise ValueError('invalid normalized box')
        left, top, right, bottom = x-w/2, y-h/2, x+w/2, y+h/2
        if min(left, top) < -1e-4 or max(right, bottom) > 1+1e-4:
            raise ValueError('box outside image')
        clipped = max(0, left), max(0, top), min(1, right), min(1, bottom)
        if clipped != (left, top, right, bottom):
            if normalizations is not None:
                normalizations['rounding_clipped_boxes'] += 1
            left, top, right, bottom = clipped
            x, y, w, h = (left+right)/2, (top+bottom)/2, right-left, bottom-top
        result.append((names[int(fields[0])], x, y, w, h))
    return sorted(result)


def group_splits(records, blocked_frames):
    parents = {item['id']: item['id'] for item in records}
    def find(key):
        while parents[key] != key:
            parents[key] = parents[parents[key]]
            key = parents[key]
        return key
    frames = {}
    for item in records:
        for member in item['members']:
            frame = member['frame']
            if frame in frames:
                parents[find(item['id'])] = find(frames[frame])
            frames[frame] = item['id']
    priority = {'train': 0, 'val': 1, 'test': 2}
    splits, blocked = {}, set()
    for item in records:
        group = find(item['id'])
        for member in item['members']:
            split = member['source_split']
            if priority[split] > priority.get(splits.get(group), -1):
                splits[group] = split
            if member['frame'] in blocked_frames:
                blocked.add(group)
    return {item['id']: None if find(item['id']) in blocked else splits[find(item['id'])] for item in records}


def crack_only_view(output):
    target = output/'crack_only'
    if target.exists():
        raise ValueError('crack-only view already exists')
    config = yaml.safe_load((output/'data.yaml').read_text(encoding='utf-8'))
    crack_index = next(index for index, name in config['names'].items() if name == 'CRACK')
    manifest = json.loads((output/'manifest.json').read_text(encoding='utf-8'))
    count = 0
    for item in manifest['images']:
        relative = Path(item['split'])/item['dataset']/(item['id']+'.jpg')
        image = target/'images'/relative
        label = target/'labels'/relative.with_suffix('.txt')
        image.parent.mkdir(parents=True, exist_ok=True)
        label.parent.mkdir(parents=True, exist_ok=True)
        os.link(output/'images'/relative, image)
        rows = ['0 '+' '.join(line.split()[1:]) for line in (output/'labels'/relative.with_suffix('.txt')).read_text().splitlines()
                if line.strip() and int(line.split()[0]) == crack_index]
        label.write_text('\n'.join(rows), encoding='utf-8')
        count += len(rows)
    view = config | {'path': str(target.resolve()), 'names': {0: 'CRACK'}}
    (target/'data.yaml').write_text(yaml.safe_dump(view), encoding='utf-8')
    (target/'target.yaml').write_text(yaml.safe_dump(view | {'val': 'images/val/msd', 'test': 'images/test/msd'}), encoding='utf-8')
    (target/'manifest.json').write_text(json.dumps({'parent_manifest_sha256': file_hash(output/'manifest.json'),
                                                 'counts': manifest['counts'], 'class_boxes': {'CRACK': count}}, indent=2), encoding='utf-8')


def prepare(source, base, output):
    if output.exists():
        raise ValueError('output exists; choose a new directory')
    base_config = yaml.safe_load((base/'data.yaml').read_text(encoding='utf-8'))
    base_names = base_config['names']
    if isinstance(base_names, list):
        base_names = dict(enumerate(base_names))
    base_manifest = json.loads((base/'manifest.json').read_text(encoding='utf-8'))
    records, known, observed = [], set(), set()
    normalizations = {'msd': Counter(), 'brick': Counter(), 'stone': Counter()}
    for entry in base_manifest['images']:
        path = base/'images'/entry['split']/(entry['sha256']+'.jpg')
        label = base/'labels'/entry['split']/(entry['sha256']+'.txt')
        rows = parse_labels(label.read_text(encoding='utf-8'), base_names, normalizations['msd'])
        with Image.open(path) as image:
            known.update((entry['sha256'], pixel_hash(image)))
        observed.update(row[0] for row in rows)
        records.append({'id': entry['sha256'], 'dataset': 'msd', 'split': entry['split'], 'labels': rows,
                        'path': path, 'source': entry['source'], 'annotation': entry['annotation']})
    codes = [code for _, code in sorted(base_names.items()) if code in observed]
    if 'CRACK' not in codes:
        raise ValueError('base dataset has no CRACK category')
    archive_path = source/'yolo'/'Brick and Stone(annotated).zip'
    excluded, blocked_frames, supplements, source_configs = [], set(), {}, {}
    blocked_pixels = set()
    with zipfile.ZipFile(archive_path) as archive:
        entries = archive.namelist()
        for material in ('brick', 'stone'):
            configs = [name for name in entries if name.endswith('/data.yaml') and f'Crack Detection {material}.' in name]
            if len(configs) != 1:
                raise ValueError(f'expected one {material} source config')
            config_path = configs[0]
            root = config_path.rsplit('/', 1)[0]
            config = yaml.safe_load(archive.read(config_path))
            source_configs[material] = config
            names = dict(enumerate(config['names']))
            for split_dir, split in (('train', 'train'), ('valid', 'val'), ('test', 'test')):
                prefix = f'{root}/{split_dir}/images/'
                images = sorted(name for name in entries if name.startswith(prefix) and name.lower().endswith(('.jpg', '.jpeg', '.png')))
                for name in images:
                    filename = Path(name).name
                    annotation = f'{root}/{split_dir}/labels/{Path(filename).stem}.txt'
                    frame = material+':'+filename.split('.rf.')[0]
                    member = {'source': name, 'annotation': annotation, 'frame': frame, 'source_split': split}
                    rows = parse_labels(archive.read(annotation).decode('utf-8'), names, normalizations[material])
                    reason = None
                    if any(row[0] != 'crack' for row in rows):
                        reason = 'unknown_source_category'
                    raw = archive.read(name)
                    with Image.open(io.BytesIO(raw)) as image:
                        digest = pixel_hash(image)
                        if image.getexif().get(274, 1) != 1:
                            reason = 'unresolved_image_orientation'
                    if digest in known:
                        reason = 'duplicate_with_msd'
                    if reason:
                        excluded.append(member | {'reason': reason})
                        blocked_frames.add(frame)
                        blocked_pixels.add(digest)
                        continue
                    rows = [('CRACK', *row[1:]) for row in rows]
                    if digest in supplements:
                        item = supplements[digest]
                        item['members'].append(member)
                        if item['labels'] != rows:
                            item['conflicting'] = True
                    else:
                        supplements[digest] = {'id': digest, 'dataset': material, 'source': name, 'annotation': annotation,
                                               'labels': rows, 'members': [member]}
        for item in supplements.values():
            if item.get('conflicting') or item['id'] in blocked_pixels:
                blocked_frames.update(member['frame'] for member in item['members'])
        assignments = group_splits(list(supplements.values()), blocked_frames)
        for item in supplements.values():
            if assignments[item['id']] is None:
                excluded.extend(member | {'reason': 'conflicting_or_excluded_frame'} for member in item['members'])
            else:
                item['split'] = assignments[item['id']]
                records.append(item)
        output.mkdir(parents=True)
        counts, by_source, class_boxes, exported = Counter(), {}, Counter(), []
        for item in records:
            relative = Path(item['split'])/item['dataset']/(item['id']+'.jpg')
            image_path, label_path = output/'images'/relative, output/'labels'/relative.with_suffix('.txt')
            image_path.parent.mkdir(parents=True, exist_ok=True)
            label_path.parent.mkdir(parents=True, exist_ok=True)
            if item['dataset'] == 'msd':
                shutil.copyfile(item['path'], image_path)
            else:
                image_path.write_bytes(archive.read(item['source']))
            label_path.write_text('\n'.join(str(codes.index(code))+' '+' '.join(f'{value:.8f}' for value in box)
                                            for code, *box in item['labels']), encoding='utf-8')
            counts[item['split']] += 1
            by_source.setdefault(item['dataset'], Counter())[item['split']] += 1
            class_boxes.update(row[0] for row in item['labels'])
            exported.append({key: item[key] for key in ('id', 'dataset', 'source', 'annotation', 'split')}
                            | {'image_sha256': file_hash(image_path), 'members': item.get('members', [])})
    config = {'path': str(output.resolve()), 'train': 'images/train', 'val': 'images/val', 'test': 'images/test', 'names': dict(enumerate(codes))}
    (output/'data.yaml').write_text(yaml.safe_dump(config), encoding='utf-8')
    target = config | {'val': 'images/val/msd', 'test': 'images/test/msd'}
    (output/'target.yaml').write_text(yaml.safe_dump(target), encoding='utf-8')
    for material in by_source:
        view = config | {split: f'images/{split}/{material}' for split in ('train', 'val', 'test')}
        (output/f'{material}.yaml').write_text(yaml.safe_dump(view), encoding='utf-8')
    manifest = {'counts': dict(counts), 'counts_by_source': by_source, 'class_boxes': dict(class_boxes),
                'base_manifest_sha256': file_hash(base/'manifest.json'), 'supplement_archive_sha256': file_hash(archive_path),
                'source_configs': source_configs, 'normalizations': normalizations, 'excluded': excluded, 'images': exported}
    (output/'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding='utf-8')
    crack_only_view(output)
    print(json.dumps({key: manifest[key] for key in ('counts', 'counts_by_source', 'class_boxes')}, ensure_ascii=False))
    print('excluded files', len(excluded))


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, default=Path(r'C:\a\school\毕设'))
    parser.add_argument('--base', type=Path, default=Path('data/yolo/dataset_msd_oriented'))
    parser.add_argument('--output', type=Path, default=Path('data/yolo/dataset_crack_augmented'))
    args = parser.parse_args()
    prepare(args.source, args.base, args.output)

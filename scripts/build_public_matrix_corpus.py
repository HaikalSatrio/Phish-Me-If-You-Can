"""Freeze public reference names and account clues before running collection tests."""

import argparse
import collections
import datetime as dt
import hashlib
import json
from pathlib import Path
import re
import time
import urllib.error
import urllib.parse
import urllib.request

USER_AGENT = 'PhishMeIfYouCan-Evaluation/1.0 (https://github.com/HaikalSatrio/Phish-Me-If-You-Can)'
PROPERTIES = {'P2003': 'instagram', 'P2002': 'x', 'P2037': 'github', 'P2013': 'facebook'}
HANDLE = re.compile(r'^[A-Za-z0-9_][A-Za-z0-9_.-]{0,99}$')


def write_new(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('x', encoding='utf-8') as stream:
        json.dump(value, stream, ensure_ascii=True, indent=2)
        stream.write('\n')


def get_json(base, params, cache):
    url = base + '?' + urllib.parse.urlencode(params)
    key = hashlib.sha256(url.encode()).hexdigest()
    path = cache / (key + '.json')
    if path.exists():
        return json.loads(path.read_text(encoding='utf-8'))
    for attempt in range(3):
        try:
            request = urllib.request.Request(url, headers={'User-Agent': USER_AGENT})
            with urllib.request.urlopen(request, timeout=45) as response:
                payload = response.read(64 * 1024 * 1024 + 1)
            if len(payload) > 64 * 1024 * 1024:
                raise ValueError('reference response exceeds 64 MiB')
            result = json.loads(payload)
            if 'error' in result:
                raise ValueError(str(result['error']))
            # The cache is local and ignored; the published corpus keeps only selected public fields.
            write_new(path, result)
            time.sleep(0.5)
            return result
        except (urllib.error.URLError, TimeoutError, ValueError) as exc:
            if attempt == 2:
                raise RuntimeError(f'reference API failed: {url}: {exc}') from exc
            time.sleep(3 * (attempt + 1))
    raise RuntimeError('unreachable')


def values(entity, prop):
    for claim in entity.get('claims', {}).get(prop, []):
        if claim.get('rank') == 'deprecated' or 'P582' in claim.get('qualifiers', {}):
            continue
        snak = claim.get('mainsnak', {})
        if snak.get('snaktype') == 'value':
            yield claim, snak['datavalue']['value']


def account_clues(entity):
    out = []
    for prop, platform in PROPERTIES.items():
        for claim, value in values(entity, prop):
            if not isinstance(value, str) or not HANDLE.fullmatch(value) or '..' in value:
                continue
            if platform == 'github' and (len(value) > 39 or '_' in value or '.' in value):
                continue
            out.append({'platform': platform, 'username': value, 'property': prop,
                        'statement_id': claim.get('id', ''),
                        'source': 'https://www.wikidata.org/wiki/' + entity['id'] + '#' + prop})
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--seeds', type=Path, default=Path('fixtures/international-name-seeds.tsv'))
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--cache', type=Path, default=Path('data/matrix-20261026/reference-cache'))
    parser.add_argument('--count', type=int, default=200)
    args = parser.parse_args()
    if args.out.exists() or not 1 <= args.count <= 400:
        parser.error('output must be new; count must be 1-400')
    args.cache.mkdir(parents=True, exist_ok=True)
    seeds = []
    for line in args.seeds.read_text(encoding='utf-8').splitlines():
        category, titles = line.split('\t', 1)
        seeds.extend((category, title) for title in titles.split('|'))
    resolved = []
    exclusions = []
    for start in range(0, len(seeds), 40):
        batch = seeds[start:start + 40]
        response = get_json('https://en.wikipedia.org/w/api.php', {
            'action': 'query', 'titles': '|'.join(title for _, title in batch), 'prop': 'pageprops',
            'ppprop': 'wikibase_item', 'redirects': 1, 'format': 'json', 'formatversion': 2, 'maxlag': 5,
        }, args.cache)
        query = response.get('query', {})
        redirects = {item['from']: item['to'] for item in query.get('normalized', []) + query.get('redirects', [])}
        pages = {item['title']: item for item in query.get('pages', [])}
        for category, original in batch:
            title, visited = original, set()
            while title in redirects and title not in visited:
                visited.add(title)
                title = redirects[title]
            page = pages.get(title, {})
            qid = page.get('pageprops', {}).get('wikibase_item')
            if not qid:
                exclusions.append({'seed': original, 'reason': 'no Wikidata entity from article'})
                continue
            resolved.append({'category': category, 'seed': original, 'title': title, 'qid': qid})
        print(f'resolved {min(start + 40, len(seeds))}/{len(seeds)} seed titles', flush=True)
    entities = {}
    ids = sorted({row['qid'] for row in resolved})
    for start in range(0, len(ids), 15):
        result = get_json('https://www.wikidata.org/w/api.php', {
            'action': 'wbgetentities', 'ids': '|'.join(ids[start:start + 15]),
            'props': 'labels|descriptions|aliases|claims|sitelinks/urls', 'languages': 'en',
            'sitefilter': 'enwiki', 'format': 'json', 'maxlag': 15,
        }, args.cache)
        entities.update(result.get('entities', {}))
        print(f'read {min(start + 15, len(ids))}/{len(ids)} reference entities', flush=True)
    eligible, seen = [], set()
    for row in resolved:
        entity = entities.get(row['qid'], {})
        if row['qid'] in seen:
            continue
        seen.add(row['qid'])
        if not any(v.get('id') == 'Q5' for _, v in values(entity, 'P31') if isinstance(v, dict)):
            exclusions.append({'seed': row['seed'], 'reason': 'entity is not an explicitly recorded person'})
            continue
        clues = account_clues(entity)
        if not clues:
            exclusions.append({'seed': row['seed'], 'reason': 'no usable non-deprecated account statement'})
            continue
        label = entity.get('labels', {}).get('en', {}).get('value', row['title'])
        names = {label, row['title']}
        names.update(item['value'] for item in entity.get('aliases', {}).get('en', []))
        for _, value in values(entity, 'P1477'):
            if isinstance(value, dict) and value.get('text'):
                names.add(value['text'])
        eligible.append({
            'id': row['qid'], 'name': label, 'category': row['category'],
            'reference_article': entity.get('sitelinks', {}).get('enwiki', {}).get('url',
                'https://en.wikipedia.org/wiki/' + urllib.parse.quote(row['title'].replace(' ', '_'))),
            'reference_entity': 'https://www.wikidata.org/wiki/' + row['qid'],
            'reference_names': sorted(names),
            'reference_description': entity.get('descriptions', {}).get('en', {}).get('value', ''),
            'country_ids': sorted({v['id'] for _, v in values(entity, 'P27') if isinstance(v, dict)}),
            'occupation_ids': sorted({v['id'] for _, v in values(entity, 'P106') if isinstance(v, dict)}),
            'available_accounts': clues,
        })
    # Round robin across categories avoids a corpus dominated by the first seed section.
    groups = collections.defaultdict(list)
    for row in eligible:
        groups[row['category']].append(row)
    selected = []
    while len(selected) < args.count and any(groups.values()):
        for category in sorted(groups):
            if groups[category] and len(selected) < args.count:
                selected.append(groups[category].pop(0))
    if len(selected) != args.count:
        raise RuntimeError(f'only {len(selected)} eligible people; add public seed names before collecting results')
    platform_counts = collections.Counter()
    for row in selected:
        clue = min(row['available_accounts'], key=lambda x: (platform_counts[x['platform']], x['platform'], x['username']))
        row['accounts'] = [clue]
        platform_counts[clue['platform']] += 1
        del row['available_accounts']
    dev_ids = {row['id'] for row in sorted(selected, key=lambda x: hashlib.sha256(('matrix-v1|' + x['id']).encode()).hexdigest())[:args.count // 5]}
    for row in selected:
        row['split'] = 'development' if row['id'] in dev_ids else 'heldout'
    selected.sort(key=lambda row: (row['split'] != 'development', row['category'], row['id']))
    write_new(args.out, {
        'version': 'public-matrix-v1', 'created_at': dt.datetime.now(dt.timezone.utc).isoformat(),
        'seed_sha256': hashlib.sha256(args.seeds.read_bytes()).hexdigest(),
        'limitations': ['Publicly documented people with usable account statements are overrepresented.',
            'Wikidata statements are reference clues, not verified ownership or current employment.',
            'Reference names and article URLs measure retrieval agreement, not identity truth.',
            'Role and employer precision require a separate source-level audit.'],
        'excluded_seeds': exclusions, 'cases': selected,
    })
    print(json.dumps({'selected': len(selected), 'platforms': platform_counts,
                      'categories': collections.Counter(x['category'] for x in selected),
                      'country_ids': len({x for row in selected for x in row['country_ids']}),
                      'development': len(dev_ids)}, ensure_ascii=True), flush=True)


if __name__ == '__main__':
    main()

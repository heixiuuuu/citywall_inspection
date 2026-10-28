import pytest

from scripts.prepare_crack_dataset import group_splits, parse_labels


def test_duplicate_frames_promote_entire_component_to_held_out_split():
    records = [
        {'id': 'a', 'members': [{'frame': 'first', 'source_split': 'train'}, {'frame': 'second', 'source_split': 'val'}]},
        {'id': 'b', 'members': [{'frame': 'second', 'source_split': 'train'}, {'frame': 'third', 'source_split': 'test'}]},
        {'id': 'c', 'members': [{'frame': 'third', 'source_split': 'train'}]},
    ]
    assert group_splits(records, set()) == {'a': 'test', 'b': 'test', 'c': 'test'}
    assert group_splits(records, {'first'}) == {'a': None, 'b': None, 'c': None}


def test_unrelated_frames_keep_source_split():
    records = [{'id': 'a', 'members': [{'frame': 'first', 'source_split': 'train'}]},
               {'id': 'b', 'members': [{'frame': 'second', 'source_split': 'val'}]}]
    assert group_splits(records, set()) == {'a': 'train', 'b': 'val'}


@pytest.mark.parametrize('row', ['0 nan .5 .2 .2', '0 .1 .1 .8 .8', '0 .5 .5 0 .2', '2 .5 .5 .2 .2'])
def test_invalid_boxes_and_unknown_indices_are_rejected(row):
    with pytest.raises(ValueError):
        parse_labels(row, {0: 'crack'})


def test_unknown_semantic_label_is_preserved_for_quarantine():
    assert parse_labels('0 .5 .5 .2 .2', {0: '4'})[0][0] == '4'


def test_small_source_rounding_overflow_is_clipped_and_counted():
    from collections import Counter
    counts = Counter()
    code, x, y, width, height = parse_labels('0 .7935 .35365 .413025 .407575', {0: 'crack'}, counts)[0]
    assert code == 'crack' and x+width/2 <= 1
    assert counts['rounding_clipped_boxes'] == 1

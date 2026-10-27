from scripts.evaluate_operating_point import match, metrics


def test_duplicate_prediction_does_not_match_truth_twice():
    truth = [("CRACK", [0, 0, 10, 10])]
    predictions = [("CRACK", .9, [0, 0, 10, 10]), ("CRACK", .8, [0, 0, 10, 10])]
    assert match(truth, predictions, .5) == {"CRACK": {"tp": 1, "fp": 1, "fn": 0}}


def test_wrong_semantic_class_and_bad_localization_remain_errors():
    truth = [("CRACK", [0, 0, 10, 10]), ("MOSS", [20, 20, 30, 30])]
    predictions = [("PLANT", .9, [0, 0, 10, 10]), ("MOSS", .8, [40, 40, 50, 50])]
    counts = match(truth, predictions, .5)
    assert counts["CRACK"] == {"tp": 0, "fp": 0, "fn": 1}
    assert counts["MOSS"] == {"tp": 0, "fp": 1, "fn": 1}
    assert counts["PLANT"] == {"tp": 0, "fp": 1, "fn": 0}


def test_empty_predictions_and_truth_are_valid():
    assert match([], [], .5) == {}
    assert metrics({"tp": 0, "fp": 0, "fn": 0})["f1"] == 0

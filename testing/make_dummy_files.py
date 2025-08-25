import numpy as np
import pandas as pd
import os

def make_dummy_csv():

    os.makedirs("test_data", exist_ok=True)

    data = {
        "A": np.random.randint(0, 100, 10),
        "B": np.random.rand(10),
        "C": np.random.choice(['foo', 'bar', 'baz'], 10)
    }
    df = pd.DataFrame(data)

    csv_path = os.path.join("test_data", "dummy.csv")
    df.to_csv(csv_path, index=False)

if __name__ == "__main__":
    make_dummy_csv()
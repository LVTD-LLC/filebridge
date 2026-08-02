def build_subscription_event(
    *,
    status,
    customer_id="cus_test",
    subscription_id="sub_test",
    metadata=None,
    cancel_at_period_end=False,
    price_id="price_test",
    **overrides,
):
    data = {
        "id": subscription_id,
        "customer": customer_id,
        "status": status,
        "cancel_at_period_end": cancel_at_period_end,
        "metadata": metadata or {},
        "items": {
            "data": [
                {
                    "price": {"id": price_id},
                    "quantity": 1,
                }
            ]
        },
    }
    data.update(overrides)
    return {
        "id": "evt_test",
        "data": {"object": data},
    }
